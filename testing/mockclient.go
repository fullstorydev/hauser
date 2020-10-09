package testing

import (
	"bytes"
	"compress/gzip"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"os"
	"sort"
	"strings"
	"time"

	"github.com/fullstorydev/hauser/client"
)

type createdExport struct {
	start    time.Time
	end      time.Time
	fields   []string
	progress int
	exportId string
}

type MockDataExportClient struct {
	data    []map[string]interface{}
	creates map[string]createdExport
}

var _ client.DataExportClient = (*MockDataExportClient)(nil)

func NewMockDataExportClient(datafile string) *MockDataExportClient {
	data := make([]map[string]interface{}, 0, 100)
	if file, err := os.Open(datafile); err != nil {
		panic(fmt.Sprintf("failed to open %s: %s", datafile, err))
	} else {
		raw, err := ioutil.ReadAll(file)
		if err != nil {
			panic(fmt.Sprintf("failed to read %s: %s", datafile, err))
		}
		if err := json.Unmarshal(raw, &data); err != nil {
			panic(fmt.Sprintf("failed to read json: %s", err))
		}

		sort.Slice(data, func(i int, j int) bool {
			return mustParseEventStartTime(data[i]).Before(mustParseEventStartTime(data[j]))
		})
	}
	return &MockDataExportClient{
		data:    data,
		creates: make(map[string]createdExport),
	}
}

func (m *MockDataExportClient) collectJsonData(start, end time.Time, fields []string) []byte {
	exportData := make([]map[string]interface{}, 0, 100)
	for i, record := range m.data {
		t := mustParseEventStartTime(record)
		if (t.Equal(start) || t.After(start)) && (t.Before(end)) {
			if len(fields) == 0 {
				exportData = append(exportData, m.data[i])
			} else {
				recordToAdd := make(map[string]interface{}, len(m.data[i]))
				for _, field := range fields {
					if field == "user_*" {
						// pick out all user vars
						for fieldName, dat := range m.data[i] {
							if strings.Index(fieldName, "user_") == 0 {
								recordToAdd[fieldName] = dat
							}
						}
					} else if field == "evt_*" {
						// pick out all event vars
						for fieldName, dat := range m.data[i] {
							if strings.Index(fieldName, "evt_") == 0 {
								recordToAdd[fieldName] = dat
							}
						}
					} else {
						if dat, ok := m.data[i][field]; ok {
							recordToAdd[field] = dat
						}
					}
				}
				exportData = append(exportData, recordToAdd)
			}
		}
	}
	raw, _ := json.Marshal(exportData)
	return raw
}

func (m *MockDataExportClient) CreateExport(start, end time.Time, fields []string) (string, error) {
	operationId := fmt.Sprintf("%d", rand.Int())
	exportId := fmt.Sprintf("%d", rand.Int())
	m.creates[operationId] = createdExport{start, end, fields, 0, exportId}
	return operationId, nil
}

func (m *MockDataExportClient) GetExportProgress(operationId string) (int, string, error) {
	if created, ok := m.creates[operationId]; !ok {
		return 0, "", client.StatusError{
			Status:     "Not Found",
			StatusCode: 404,
			RetryAfter: 0,
			Body:       nil,
		}
	} else {
		prog := created.progress

		if created.progress < 100 {
			created.progress += int(rand.Float32() * 100)
			// In lieu of a MaxInt function
			if created.progress > 100 {
				created.progress = 100
			}
		}

		m.creates[operationId] = created

		if prog == 100 {
			return prog, created.exportId, nil
		}
		return prog, "", nil
	}
}

func (m *MockDataExportClient) GetExport(exportId string) (io.ReadCloser, error) {
	for _, created := range m.creates {
		if created.exportId == exportId {
			raw := m.collectJsonData(created.start, created.end, created.fields)
			var b bytes.Buffer
			gw := gzip.NewWriter(&b)
			if _, err := io.Copy(gw, bytes.NewReader(raw)); err != nil {
				return nil, err
			}
			return ioutil.NopCloser(&b), gw.Close()
		}
	}
	return nil, client.StatusError{
		Status:     "Not Found",
		StatusCode: 404,
		RetryAfter: 0,
		Body:       nil,
	}
}

func mustParseEventStartTime(record map[string]interface{}) time.Time {
	if v, ok := record["EventStart"]; !ok {
		panic("EventStart didn't exist for record")
	} else if str, ok := v.(string); !ok {
		panic("Invalid format for EventStart")
	} else if eventTime, err := time.Parse(time.RFC3339Nano, str); err != nil {
		panic("Invalid format for EventStart")
	} else {
		return eventTime
	}
}
