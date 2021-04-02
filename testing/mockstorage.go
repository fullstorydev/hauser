package testing

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"time"

	"github.com/fullstorydev/hauser/warehouse"
)

type MockStorage struct {
	Syncs         []time.Time
	UploadedFiles map[string][]byte
	DeletedFiles  []string
	FilePrefix    string
}

var _ warehouse.Storage = (*MockStorage)(nil)

func NewMockStorage() *MockStorage {
	return &MockStorage{
		Syncs:         nil,
		UploadedFiles: make(map[string][]byte),
		DeletedFiles:  nil,
		FilePrefix:    "prefix/",
	}
}

func (m *MockStorage) LastSyncPoint(_ context.Context) (time.Time, error) {
	var max time.Time
	for i, s := range m.Syncs {
		if i == 0 || s.After(max) {
			max = s
		}
	}
	return max, nil
}

func (m *MockStorage) SaveSyncPoint(ctx context.Context, endTime time.Time) error {
	m.Syncs = append(m.Syncs, endTime)
	return nil
}

func (m *MockStorage) SaveFile(_ context.Context, name string, reader io.Reader) (string, error) {
	data, err := ioutil.ReadAll(reader)
	if err != nil {
		return "", err
	}
	m.UploadedFiles[name] = data
	return m.GetFileReference(name), nil
}

func (m *MockStorage) ReadFile(_ context.Context, name string) (io.Reader, error) {
	if data, ok := m.UploadedFiles[name]; !ok {
		return nil, warehouse.ErrFileNotFound
	} else {
		return bytes.NewReader(data), nil
	}
}

func (m *MockStorage) DeleteFile(_ context.Context, path string) error {
	m.DeletedFiles = append(m.DeletedFiles, path)
	return nil
}

func (m *MockStorage) GetFileReference(name string) string {
	return fmt.Sprintf("mock://%s", name)
}

func (m *MockStorage) GetFilePrefix() string {
	return m.FilePrefix
}
