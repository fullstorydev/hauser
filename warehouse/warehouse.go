package warehouse

import (
	"fmt"
	"strings"
	"time"

	"github.com/fullstorydev/hauser/client"
)

type Warehouse interface {
	LastSyncPoint() (time.Time, error)
	SaveSyncPoints(bundles ...client.ExportMeta) error
	LoadToWarehouse(filename string, bundles ...client.ExportMeta) error
	ValueToString(val interface{}, isTime bool) string
	GetExportTableColumns() []string
	EnsureCompatibleExportTable() error
	UploadFile(name string) (string, error)
	DeleteFile(path string)
	GetUploadFailedMsg(filename string, err error) string
	IsUploadOnly() bool
}

const RFC3339Micro = "2006-01-02T15:04:05.999999Z07:00"

// valueToString is a common interface method that implementations use to perform value to string conversion
func valueToString(val interface{}, isTime bool) string {
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
