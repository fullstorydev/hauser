package warehouse

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"
)

// BundleEvent represents a single event, as it's structured inside a FullStory export bundle.
type BundleEvent struct {
	EventCustomName        string
	EventStart             time.Time
	EventType              string
	EventTargetText        string
	EventTargetSelectorTok string
	EventModFrustrated     int64
	EventModDead           int64
	EventModError          int64
	EventModSuspicious     int64
	IndvId                 int64
	PageClusterId          int64
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
	CustomVars             string
	LoadDomContentTime     int64
	LoadFirstPaintTime     int64
	LoadEventTime          int64
}

// syncTable represents all the fields that should appear in the table used to track which bundles have been synced.
type syncTable struct {
	ID            int64
	Processed     time.Time
	BundleEndTime time.Time
}

// WarehouseField contains metadata for a field/column in the warehouse.
type WarehouseField struct {
	Name   string
	DBType string
}

// BundleField contains metadata for an attribute on an event object in an export bundle JSON document.
type BundleField struct {
	Name        string
	IsTime      bool
	IsCustomVar bool
}

func (f WarehouseField) String() string {
	return fmt.Sprintf("%s %s", f.Name, f.DBType)
}

type Schema []WarehouseField

func (s Schema) String() string {
	ss := make([]string, len(s))
	for i, f := range s {
		ss[i] = f.String()
	}
	return strings.Join(ss, ",")
}

type FieldTypeMapper map[string]string

// BundleFields retrieves information about the data fields in a FullStory export bundle. A bundle is
// a JSON document that contains an array of event data objects. The fields in the bundle schema
// reflect the attributes of those event JSON objects.
func BundleFields() map[string]BundleField {
	t := reflect.TypeOf(BundleEvent{})
	result := make(map[string]BundleField, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		result[strings.ToLower(t.Field(i).Name)] = BundleField{
			Name:        t.Field(i).Name,
			IsTime:      t.Field(i).Type == reflect.TypeOf(time.Time{}),
			IsCustomVar: t.Field(i).Name == "CustomVars",
		}
	}
	return result
}

// ExportTableSchema retrieves information about the fields in the warehouse table into which data will
// finally be loaded.
func ExportTableSchema(ftm FieldTypeMapper) Schema {
	// for now, the export table schema contains the same set of fields as the raw bundles
	return structToSchema(BundleEvent{}, ftm)
}

func SyncTableSchema(ftm FieldTypeMapper) Schema {
	return structToSchema(syncTable{}, ftm)
}

func structToSchema(i interface{}, ftm FieldTypeMapper) Schema {
	t := reflect.TypeOf(i)
	result := make(Schema, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		result[i] = WarehouseField{
			Name:   t.Field(i).Name,
			DBType: convertType(ftm, t.Field(i).Type),
		}
	}
	return result
}

func convertType(ftm FieldTypeMapper, t reflect.Type) string {
	dbtype, ok := ftm[t.String()]
	if !ok {
		log.Fatalf("Type %v is not present in FieldTypeMapper", t)
	}
	return dbtype
}
