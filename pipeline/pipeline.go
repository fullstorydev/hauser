package pipeline

import (
	"encoding/csv"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"time"

	"github.com/fullstorydev/hauser/config"
	"github.com/nishanths/fullstory"
)

// A Pipeline downloads data exports from fullstory.com and saves them to local disk
type Pipeline struct {
	transformRecord func(map[string]interface{}) ([]string, error)
	metaTime        time.Time
	metaCh          chan fullstory.ExportMeta
	mgCh            chan []fullstory.ExportMeta
	saveCh          chan SavedExport
	errCh           chan error
	quitCh          chan struct{}
	conf            *config.Config
}

// NewPipeline returns a new Pipeline base on the configuration provided
func NewPipeline(conf *config.Config, transformRecord func(map[string]interface{}) ([]string, error)) *Pipeline {
	return &Pipeline{
		conf:            conf,
		transformRecord: transformRecord,
		metaCh:          make(chan fullstory.ExportMeta),
		mgCh:            make(chan []fullstory.ExportMeta),
		saveCh:          make(chan SavedExport),
		errCh:           make(chan error),
		quitCh:          make(chan struct{}),
	}
}

// Start begins pipeline downloading and processing bundles after the provided time. This function returns a channel
// that will contain the filenames to which the data was saved and a channel that contains any errors that occur in the
// pipeline. These must be retrieved from the channel to continue processing.
func (p *Pipeline) Start(startTime time.Time) (chan SavedExport, chan error) {
	p.metaTime = startTime
	go p.metaFetcher()

	if p.conf.GroupFilesByDay {
		go p.metaDayGrouper()
	} else {
		go p.metaNoopGrouper()
	}

	go p.exportSaver()
	return p.saveCh, p.errCh
}

// Stop is used to stop the pipeline from processing any more exports
func (p *Pipeline) Stop() {
	close(p.quitCh)
	close(p.metaCh)
	close(p.saveCh)
	close(p.errCh)
}

func (p *Pipeline) metaFetcher() {
	fs := getFSClient(p.conf)
	for {
		exportList, err := fs.ExportList(p.metaTime)
		if err != nil {
			p.errCh <- fmt.Errorf("could not fetch export list: %s", err)
			return
		}
		for _, meta := range exportList {
			select {
			case p.metaCh <- meta:
			case <-p.quitCh:
				return
			}
		}
		if len(exportList) == 0 {
			log.Printf("No exports pending; sleeping %s", p.conf.CheckInterval.Duration)
			time.Sleep(p.conf.CheckInterval.Duration)
		}
	}
}

func (p *Pipeline) metaNoopGrouper() {
	for meta := range p.metaCh {
		p.mgCh <- []fullstory.ExportMeta{meta}
	}
}

func (p *Pipeline) metaDayGrouper() {
	var groupDay time.Time
	var metaList []fullstory.ExportMeta
	for meta := range p.metaCh {

		dataDay := meta.Start.UTC().Truncate(24 * time.Hour)
		if groupDay.Before(dataDay) {
			select {
			case p.mgCh <- metaList:
			case <-p.quitCh:
				return
			}
			metaList = []fullstory.ExportMeta{meta}
			groupDay = dataDay
		} else {
			metaList = append(metaList, meta)
		}
	}
}

func (p *Pipeline) exportSaver() {
	fs := getFSClient(p.conf)
	for metaList := range p.mgCh {
		if len(metaList) == 0 {
			continue
		}

		// CSV is the default format
		ext := "csv"
		if p.conf.SaveAsJson {
			ext = "json"
		}

		fn := fmt.Sprintf("export_%v.%s", metaList[0].ID, ext)
		fn = filepath.Join(p.conf.TmpDir, fn)

		out, err := os.Create(fn)
		if err != nil {
			p.errCh <- fmt.Errorf("error creating temp file: %s", err)
			return
		}

		for _, meta := range metaList {
			data, err := getExportData(fs, meta)
			if err != nil {
				p.errCh <- fmt.Errorf("error fetching data export: %s", err)
				return
			}

			if p.conf.SaveAsJson {
				p.saveToJSON(out, data)
			} else {
				p.saveToCSV(out, data)
			}
		}
		out.Close()
		p.saveCh <- SavedExport{
			Filename: fn,
			Meta:     metaList,
		}
	}
}

func (p *Pipeline) saveToCSV(w io.Writer, d *ExportData) {
	csvOut := csv.NewWriter(w)
	decoder, err := d.GetJSONDecoder()
	if err != nil {
		p.errCh <- fmt.Errorf("error creating export decoder: %s", err)
		return
	}

	// skip array open delimiter
	if _, err := decoder.Token(); err != nil {
		p.errCh <- fmt.Errorf("failed json decode of array open token: %s", err)
		return
	}

	for decoder.More() {
		var r Record
		decoder.Decode(&r)
		line, err := p.transformRecord(r)
		if err != nil {
			log.Printf("failed object transform, bundle %d; skipping record. %s", d.meta.ID, err)
			continue
		}
		if err := csvOut.Write(line); err != nil {
			p.errCh <- fmt.Errorf("error writing CSV: %s", err)
		}
	}

	if _, err := decoder.Token(); err != nil {
		p.errCh <- fmt.Errorf("failed json decode of array close token: %s", err)
		return
	}
}

func (p *Pipeline) saveToJSON(w io.Writer, d *ExportData) {
	stream, err := d.GetRawReader()
	if err != nil {
		p.errCh <- fmt.Errorf("error creating export reader: %s", err)
		return
	}
	io.Copy(w, stream)
}
