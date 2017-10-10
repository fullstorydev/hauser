package warehouse

import (
	"time"

	"github.com/nishanths/fullstory"
)

type Warehouse interface {
	ExportTableSchema() Schema
	LastSyncPoint() (time.Time, error)
	SaveSyncPoints(bundles []fullstory.ExportMeta) error
	LoadToWarehouse(filename string) error
	ValueToString(val interface{}, f Field) string
}
