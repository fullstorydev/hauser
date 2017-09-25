package warehouse

import "time"

type Warehouse interface {
	LastSyncPoint() (time.Time, error)
	SaveSyncPoint(id int, stop time.Time) error
	LoadToWarehouse(filename string) error
	// VarCharMaxLen returns the maximum length of a varchar type field, if applicable. If there are no length limits,
	// ok will be false.
	VarCharMaxLen() (maxlen int, ok bool)
}
