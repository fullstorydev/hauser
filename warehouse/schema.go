package warehouse

import (
	"reflect"
	"time"
)

type Field interface {
	Name() string
	DataType() reflect.Type
	IsTime() bool
}

type dataField struct {
	name     string
	dataType reflect.Type
	isTime   bool
}

func (f *dataField) Name() string {
	return f.name
}

func (f *dataField) DataType() reflect.Type {
	return f.dataType
}

func (f *dataField) IsTime() bool {
	return f.isTime
}

func newDataField(f reflect.StructField) *dataField {
	return &dataField{
		name:     f.Name,
		dataType: f.Type,
		isTime:   (f.Type == reflect.TypeOf(time.Time{})),
	}
}

var (
	ExportTableSchema = toFieldSlice(reflect.TypeOf(exportSchema{}))
	CustomVars        = toFieldSlice(reflect.TypeOf(customVars{}))[0]
	SyncTableSchema   = toFieldSlice(reflect.TypeOf(syncTable{}))
)

type exportSchema struct {
	EventStart             time.Time
	EventType              string
	EventTargetText        string
	EventTargetSelectorTok string
	EventModFrustrated     int64
	EventModDead           int64
	EventModError          int64
	EventModSuspicious     int64
	IndvId                 int64
	PageUrl                string
	PageDuration           int64
	PageActiveDuration     int64
	PageRefererUrl         string
	PageLatLong            string
	PageAgent              string
	PageIp                 string
	PageBrowser            string
	PageDevice             string
	PageOperatingSystem    string
	PageNumInfos           int64
	PageNumWarnings        int64
	PageNumErrors          int64
	SessionId              int64
	PageId                 int64
	UserAppKey             string
	UserEmail              string
	UserDisplayName        string
	UserId                 int64
}

type customVars struct {
	CustomVars string
}

type syncTable struct {
	ID            int64
	Processed     time.Time
	BundleEndTime time.Time
}

func toFieldSlice(t reflect.Type) []Field {
	result := make([]Field, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		result[i] = newDataField(t.Field(i))
	}
	return result
}
