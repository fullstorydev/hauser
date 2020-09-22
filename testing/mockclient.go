package testing

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/rand"
	"net/http"
	"os"
	"sort"
	"time"

	"github.com/fullstorydev/hauser/client"
)

var (
	// In order to keep tests consistent, pretend that now is constant.
	now = time.Date(2020, 9, 1, 0, 0, 0, 0, time.UTC)
)

const maxListCount = 20

type MockDataExportClient struct {
	// Determines the time range for the requested bundle ID.
	freqSetting int32
	data        []map[string]interface{}
	creates     map[string]struct {
		start    time.Time
		end      time.Time
		progress int
		exportId string
	}
}

var _ client.DataExportClient = (*MockDataExportClient)(nil)

func NewMockDataExportClient(freqSetting int32, datafile string) *MockDataExportClient {
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
		freqSetting: freqSetting,
		data:        data,
	}
}

func (m *MockDataExportClient) ExportList(start time.Time) ([]client.ExportMeta, error) {
	bundleDuration := time.Duration(m.freqSetting) * (30 * time.Minute)
	// Let's pretend that it's possible there is data up to a day before there is actually data
	startToUse := mustParseEventStartTime(m.data[0]).Add(-24 * time.Hour)
	if start.After(startToUse) {
		startToUse = start
	}
	bundleStart := startToUse.Truncate(bundleDuration)
	bundleStop := bundleStart.Add(bundleDuration)
	meta := make([]client.ExportMeta, 0, maxListCount)
	for bundleStop.Before(now) && len(meta) <= maxListCount {
		meta = append(meta, client.ExportMeta{
			Start: bundleStart,
			Stop:  bundleStop,
			ID:    int(bundleStart.Unix()*100 + int64(m.freqSetting)),
		})
		bundleStart = bundleStop
		bundleStop = bundleStop.Add(bundleDuration)
	}
	return meta, nil
}

func (m *MockDataExportClient) ExportData(id int, _ ...func(r *http.Request)) (client.ExportData, error) {
	bundleStart := time.Unix(int64(id/100), 0).UTC()
	bucketSize := id % 100
	if bucketSize <= 0 {
		return nil, errors.New("bad export id")
	}
	bundleEnd := bundleStart.Add(time.Duration(bucketSize) * 30 * time.Minute)
	raw := m.collectJsonData(bundleStart, bundleEnd)
	return ioutil.NopCloser(bytes.NewReader(raw)), nil
}

func (m *MockDataExportClient) collectJsonData(start, end time.Time) []byte {
	exportData := make([]map[string]interface{}, 0, 100)
	for i, record := range m.data {
		t := mustParseEventStartTime(record)
		if (t.Equal(start) || t.After(start)) && (t.Before(end)) {
			exportData = append(exportData, m.data[i])
		}
	}
	raw, _ := json.Marshal(exportData)
	return raw
}

func (m *MockDataExportClient) CreateExport(start, end time.Time) (string, error) {
	id := fmt.Sprintf("%d", rand.Int())
	m.creates[id] = struct {
		start    time.Time
		end      time.Time
		progress int
		exportId string
	}{start, end, 0, fmt.Sprintf("%d", rand.Int())}
	return id, nil
}

func (m *MockDataExportClient) GetExportProgress(operationId string) (int, bool, error) {
	if created, ok := m.creates[operationId]; !ok {
		return 0, false, client.StatusError{
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
		return prog, prog == 100, nil
	}
}

func (m *MockDataExportClient) GetExport(operationId string) (io.ReadCloser, error) {
	if created, ok := m.creates[operationId]; !ok {
		return nil, client.StatusError{
			Status:     "Not Found",
			StatusCode: 404,
			RetryAfter: 0,
			Body:       nil,
		}
	} else {
		raw := m.collectJsonData(created.start, created.end)
		return ioutil.NopCloser(bytes.NewReader(raw)), nil
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
