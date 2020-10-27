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
)

const (
	timestampFile string = ".sync.hauser"
)

var (
	ErrFileNotFound = errors.New("file not found")
)

type Syncable interface {
	LastSyncPoint(ctx context.Context) (time.Time, error)
	SaveSyncPoint(ctx context.Context, endTime time.Time) error
}

type Storage interface {
	Syncable
	SaveFile(ctx context.Context, name string, reader io.Reader) (string, error)
	ReadFile(ctx context.Context, name string) (io.Reader, error)
	DeleteFile(ctx context.Context, name string) error
	GetFileReference(name string) string
}

type Database interface {
	Syncable
	LoadToWarehouse(storageRef string, start time.Time) error
	ValueToString(val interface{}, isTime bool) string
	GetExportTableColumns() []string

	// InitExportTable should attempt to create the table in the database. If the table doesn't exist, the provided
	// schema should be applied to the table and this function should return `true`, assuming an error didn't occur.
	// If the table existed, this should return false to signal that follow-up schema validation is necessary.
	InitExportTable(Schema) (bool, error)

	// ApplyExportSchema will attempt to update the schema in the database to the provided schema.
	// The provided schema must be compatible, or this will fail. Compatible schemas will have existing columns
	// in the same order as they are currently ordered in the table and can also add new columns to the end.
	ApplyExportSchema(Schema) error
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

// SyncViaStorageMixin provides a default implementation for the Syncable interface.
type SyncViaStorageMixin struct {
	storage Storage
}

var _ Syncable = (*SyncViaStorageMixin)(nil)

func (s SyncViaStorageMixin) LastSyncPoint(ctx context.Context) (time.Time, error) {
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

func (s SyncViaStorageMixin) SaveSyncPoint(ctx context.Context, endTime time.Time) error {
	if endTime.IsZero() {
		panic("An end time of zero is not a valid sync point SaveSyncPoints")
	}
	t := endTime.UTC().Format(time.RFC3339)
	r := bytes.NewReader([]byte(t))
	_, err := s.storage.SaveFile(ctx, timestampFile, r)
	return err
}
