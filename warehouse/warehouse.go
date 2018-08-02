package warehouse

import (
	"time"

	"github.com/nishanths/fullstory"
)

type Warehouse interface {
	LastSyncPoint() (time.Time, error)
	SaveSyncPoints(bundles ...fullstory.ExportMeta) error
	LoadToWarehouse(filename string, bundles ...fullstory.ExportMeta) error
	ValueToString(val interface{}, isTime bool) string
	GetExportTableColumns() []string
	EnsureCompatibleExportTable() error
}
