package testing

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"time"

	"github.com/fullstorydev/hauser/client"
	"github.com/fullstorydev/hauser/warehouse"
)

var (
	MockTypeMap = warehouse.FieldTypeMapper{
		"int64":     "BIGINT",
		"string":    "VARCHAR(max)",
		"time.Time": "TIMESTAMP",
	}
)

type MockWarehouse struct {
	schema        warehouse.Schema
	Initialized   bool
	Syncs         []client.ExportMeta
	LoadedFiles   []string
	UploadedFiles map[string][]byte
	DeletedFiles  []string
}

var _ warehouse.Warehouse = (*MockWarehouse)(nil)

func NewMockWarehouse() *MockWarehouse {
	return &MockWarehouse{
		schema:        nil,
		Initialized:   false,
		Syncs:         nil,
		LoadedFiles:   nil,
		UploadedFiles: make(map[string][]byte),
		DeletedFiles:  nil,
	}
}

func (m *MockWarehouse) LastSyncPoint() (time.Time, error) {
	var max time.Time
	for i, s := range m.Syncs {
		if i == 0 || s.Stop.After(max) {
			max = s.Stop
		}
	}
	return max, nil
}

func (m *MockWarehouse) SaveSyncPoints(bundles ...client.ExportMeta) error {
	m.Syncs = append(m.Syncs, bundles...)
	return nil
}

func (m *MockWarehouse) LoadToWarehouse(filename string, _ ...client.ExportMeta) error {
	isUploaded := false
	for name := range m.UploadedFiles {
		if filename == name {
			isUploaded = true
			break
		}
	}
	if !isUploaded {
		return fmt.Errorf("no such file: %s", filename)
	}
	m.LoadedFiles = append(m.LoadedFiles, filename)
	return nil
}

func (m *MockWarehouse) ValueToString(val interface{}, isTime bool) string {
	s := fmt.Sprintf("%v", val)
	if isTime {
		t, _ := time.Parse(time.RFC3339Nano, s)
		return t.Format(warehouse.RFC3339Micro)
	}
	return s
}

func (m *MockWarehouse) GetExportTableColumns() []string {
	cols := make([]string, 0, len(m.schema))
	for _, f := range m.schema {
		cols = append(cols, f.Name)
	}
	return cols
}

func (m *MockWarehouse) EnsureCompatibleExportTable() error {
	m.schema = warehouse.ExportTableSchema(MockTypeMap)
	m.Initialized = true
	return nil
}

func (m *MockWarehouse) UploadFile(name string) (string, error) {
	_, objName := filepath.Split(name)
	f, err := os.Open(name)
	if err != nil {
		return "", err
	}
	data, err := ioutil.ReadAll(f)
	if err != nil {
		return "", err
	}
	m.UploadedFiles[objName] = data
	return objName, nil
}

func (m *MockWarehouse) DeleteFile(path string) {
	m.DeletedFiles = append(m.DeletedFiles, path)
}

func (m *MockWarehouse) GetUploadFailedMsg(filename string, err error) string {
	return fmt.Sprintf("Failed to upload %s to MockWarehouse: %s", filename, err)
}

func (m MockWarehouse) IsUploadOnly() bool {
	return false
}
