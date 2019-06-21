package pipeline

import (
	"bytes"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"os"
	"time"

	"github.com/fullstorydev/hauser/config"
	"github.com/nishanths/fullstory"
)

// A Pipeline downloads data exports from fullstory.com and saves them to local disk
type Pipeline struct {
	transformRecord func(map[string]interface{}) ([]string, error)
	metaTime        time.Time
	metaCh          chan fullstory.ExportMeta
	expCh           chan ExportData
	saveCh          chan SavedExport
	recsCh          chan RecordGroup
	quitCh          chan interface{}
	conf            *config.Config
}

// NewPipeline returns a new Pipeline base on the configuration provided
func NewPipeline(conf *config.Config, transformRecord func(map[string]interface{}) ([]string, error)) *Pipeline {
	return &Pipeline{
		conf:            conf,
		metaTime:        conf.StartTime,
		transformRecord: transformRecord,
		metaCh:          make(chan fullstory.ExportMeta),
		expCh:           make(chan ExportData),
		recsCh:          make(chan RecordGroup),
		saveCh:          make(chan SavedExport),
		quitCh:          make(chan interface{}),
	}
}

// Start begins pipeline downloading and processing. This function returns a channel that will contain the
// filenames to which the data was saved. These must be retrieved from the channel to continue processing.
func (p *Pipeline) Start() chan SavedExport {
	go p.metaFetcher()
	go p.exportFetcher()
	go p.recordGrouper()
	go p.recordsSaver()
	return p.saveCh
}

// Stop is used to stop the pipeline from processing any more exports
func (p *Pipeline) Stop() {
	close(p.expCh)
	close(p.metaCh)
	close(p.recsCh)
	close(p.saveCh)
}

func (p *Pipeline) metaFetcher() {
	for {
		fs := getFSClient(p.conf)
		exportList, err := fs.ExportList(p.metaTime)
		if err != nil {
			log.Printf("Could not fetch export list: %s", err)
			continue
		}
		for _, meta := range exportList {
			select {
			case p.metaCh <- meta:
				p.metaTime = meta.Stop
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

func (p *Pipeline) exportFetcher() {
	for {
		select {
		case meta := <-p.metaCh:
			fs := getFSClient(p.conf)
			data, err := getDataWithRetry(fs, meta)
			if err != nil {
				log.Printf("Error fetching data export: %s", err)
				continue
			}
			select {
			case p.expCh <- data:
				continue
			case <-p.quitCh:
				return
			}
		case <-p.quitCh:
			return
		}
	}
}

func (p *Pipeline) recordGrouper() {
	var recs []Record
	var metas []fullstory.ExportMeta
	var groupDay time.Time
	for {
		select {
		case data := <-p.expCh:
			newRecs, err := data.GetRecords()
			if err != nil {
				log.Printf("Could not decode records: %s", err)
			}
			if p.conf.GroupFilesByDay {
				dataDay := data.meta.Start.UTC().Truncate(24 * time.Hour)
				if groupDay.Before(dataDay) {
					if len(recs) > 0 {
						p.recsCh <- RecordGroup{
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
				p.recsCh <- RecordGroup{
					records: newRecs,
					bundles: []fullstory.ExportMeta{data.meta},
				}
			}
		}
	}
}

func (p *Pipeline) recordsSaver() {
	for {
		select {
		case rg := <-p.recsCh:

			// CSV is the default format
			ext := "csv"
			if p.conf.SaveAsJson {
				ext = "json"
			}

			fn := fmt.Sprintf("./export_%v.%s", rg.bundles[0].Start.Format(time.RFC3339), ext)

			out, err := os.Create(fn)
			if err != nil {
				log.Printf("Error creating temp file: %s", err)
				continue
			}

			var dataSrc io.Reader
			if p.conf.SaveAsJson {
				var err error
				var jsonRecs []byte
				if p.conf.PrettyJSON {
					jsonRecs, err = json.MarshalIndent(rg.records, "", " ")
				} else {
					jsonRecs, err = json.Marshal(rg.records)
				}
				if err != nil {
					log.Printf("Error marshaling records: %s", err)
					continue
				}
				dataSrc = bytes.NewReader(jsonRecs)
				io.Copy(out, dataSrc)
			} else {
				csvOut := csv.NewWriter(out)
				for _, rec := range rg.records {
					line, err := p.transformRecord(rec)
					if err != nil {
						log.Printf("error transforming recodes: %s", err)
						continue
					}
					csvOut.Write(line)
				}
				if p.transformRecord != nil {

				}
			}
			out.Close()
			p.saveCh <- SavedExport{Filename: fn, Meta: rg.bundles}
		case <-p.quitCh:
			return
		}
	}
}
