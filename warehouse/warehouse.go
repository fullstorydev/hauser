package warehouse

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"strings"
	"time"

	"github.com/fullstorydev/hauser/client"
)

const (
	timestampFile string = ".sync.hauser"
)

var (
	ErrFileNotFound = errors.New("file not found")
)

type Syncable interface {
	LastSyncPoint(ctx context.Context) (time.Time, error)
	SaveSyncPoints(ctx context.Context, bundles ...client.ExportMeta) error
}

type Storage interface {
	Syncable
	SaveFile(ctx context.Context, name string, reader io.Reader) error
	ReadFile(ctx context.Context, name string) (io.Reader, error)
	DeleteFile(ctx context.Context, name string) error
	GetFileReference(name string) string
}

type Database interface {
	Syncable
	LoadToWarehouse(storageRef string, bundles ...client.ExportMeta) error
	ValueToString(val interface{}, isTime bool) string
	GetExportTableColumns() []string
	EnsureCompatibleExportTable() error
}

const RFC3339Micro = "2006-01-02T15:04:05.999999Z07:00"

type ValueToStringFn func(val interface{}, isTime bool) string

// ValueToString is a common interface method that implementations use to perform value to string conversion
func ValueToString(val interface{}, isTime bool) string {
	s := fmt.Sprintf("%v", val)
	if isTime {
		t, _ := time.Parse(time.RFC3339Nano, s)
		return t.Format(RFC3339Micro)
	}

	s = strings.Replace(s, "\n", " ", -1)
	s = strings.Replace(s, "\r", " ", -1)
	s = strings.Replace(s, "\x00", "", -1)

	return s
}

// StorageMixin provides a default implementation for the Syncable interface.
type StorageMixin struct {
	storage Storage
}

var _ Syncable = (*StorageMixin)(nil)

func (s StorageMixin) LastSyncPoint(ctx context.Context) (time.Time, error) {
	r, err := s.storage.ReadFile(ctx, timestampFile)
	if err != nil {
		if err == ErrFileNotFound {
			// This is alright, we just haven't created one yet. Return the zero time value.
			return time.Time{}, nil
		}
		return time.Time{}, fmt.Errorf("failed to create sync file reader: %s", err)
	}

	if tBytes, err := ioutil.ReadAll(r); err != nil {
		return time.Time{}, fmt.Errorf("failed to read sync file: %s", err)
	} else {
		return time.Parse(time.RFC3339, string(tBytes))
	}
}

func (s StorageMixin) SaveSyncPoints(ctx context.Context, bundles ...client.ExportMeta) error {
	if len(bundles) == 0 {
		panic("Zero-length bundle list passed to SaveSyncPoints")
	}
	t := bundles[len(bundles)-1].Stop.UTC().Format(time.RFC3339)
	r := bytes.NewReader([]byte(t))
	return s.storage.SaveFile(ctx, timestampFile, r)
}
