package pipeline

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
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
	metaCh          chan *fullstory.ExportMeta
	expCh           chan *ExportData
	saveCh          chan *SavedExport
	recsCh          chan *RecordGroup
	errCh           chan error
	conf            *config.Config
}

// NewPipeline returns a new Pipeline base on the configuration provided
func NewPipeline(conf *config.Config, transformRecord func(map[string]interface{}) ([]string, error)) *Pipeline {
	return &Pipeline{
		conf:            conf,
		transformRecord: transformRecord,
		metaCh:          make(chan *fullstory.ExportMeta),
		expCh:           make(chan *ExportData),
		recsCh:          make(chan *RecordGroup),
		saveCh:          make(chan *SavedExport),
		errCh:           make(chan error),
	}
}

// Start begins pipeline downloading and processing bundles after the provided time. This function returns a channel
// that will contain the filenames to which the data was saved and a channel that contains any errors that occur in the
// pipeline. These must be retrieved from the channel to continue processing.
func (p *Pipeline) Start(startTime time.Time) (chan *SavedExport, chan error) {
	p.metaTime = startTime
	go p.metaFetcher()
	go p.exportFetcher()
	go p.recordGrouper()
	go p.recordsSaver()
	return p.saveCh, p.errCh
}

// Stop is used to stop the pipeline from processing any more exports
func (p *Pipeline) Stop() {
	close(p.expCh)
	close(p.metaCh)
	close(p.recsCh)
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
			p.metaCh <- &meta
			p.metaTime = meta.Stop
		}
		if len(exportList) == 0 {
			log.Printf("No exports pending; sleeping %s", p.conf.CheckInterval.Duration)
			time.Sleep(p.conf.CheckInterval.Duration)
		}
	}
}

func (p *Pipeline) exportFetcher() {
	fs := getFSClient(p.conf)
	for meta := range p.metaCh {
		data, err := getExportData(fs, *meta)
		if err != nil {
			p.errCh <- fmt.Errorf("error fetching data export: %s", err)
			return
		}
		p.expCh <- data
	}
}

func (p *Pipeline) recordGrouper() {
	var recs []Record
	var metas []fullstory.ExportMeta
	var groupDay time.Time

	for data := range p.expCh {
		newRecs, err := data.GetRecords()
		if err != nil {
			p.errCh <- fmt.Errorf("could not decode records: %s", err)
			return
		}

		if p.conf.GroupFilesByDay {
			dataDay := data.meta.Start.UTC().Truncate(24 * time.Hour)
			if groupDay.Before(dataDay) {
				if len(recs) > 0 {
					p.recsCh <- &RecordGroup{
						records: recs,
						bundles: metas,
					}
					recs = recs[:0]
					metas = metas[:0]
				}
				groupDay = dataDay
			}
			recs = append(recs, newRecs...)
			metas = append(metas, data.meta)
		} else {
			if len(newRecs) == 0 {
				continue
			}
			p.recsCh <- &RecordGroup{
				records: newRecs,
				bundles: []fullstory.ExportMeta{data.meta},
			}
		}
	}
}

func (p *Pipeline) recordsSaver() {
	for rg := range p.recsCh {

		// CSV is the default format
		ext := "csv"
		if p.conf.SaveAsJson {
			ext = "json"
		}

		fn := fmt.Sprintf("export_%v.%s", rg.bundles[0].ID, ext)
		fn = filepath.Join(p.conf.TmpDir, fn)

		out, err := os.Create(fn)
		if err != nil {
			p.errCh <- fmt.Errorf("error creating temp file: %s", err)
			return
		}

		var dataSrc io.Reader
		if p.conf.SaveAsJson {
			jsonRecs, err := json.Marshal(rg.records)
			if err != nil {
				p.errCh <- fmt.Errorf("error marshaling records: %s", err)
				return
			}
			dataSrc = bytes.NewReader(jsonRecs)
			io.Copy(out, dataSrc)
		} else {
			csvOut := csv.NewWriter(out)
			for _, rec := range rg.records {
				line, err := p.transformRecord(rec)
				if err != nil {
					p.errCh <- fmt.Errorf("error transforming recodes: %s", err)
					return
				}
				err = csvOut.Write(line)
				if err != nil {
					p.errCh <- fmt.Errorf("error writing CSV: %s", err)
				}
			}
		}
		err = out.Close()
		if err != nil {
			p.errCh <- fmt.Errorf("error closing file: %s", err)
			return
		}
		p.saveCh <- &SavedExport{Filename: fn, Meta: rg.bundles}
	}
}
