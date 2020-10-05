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

type BaseExportFields struct {
	IndvId              int64
	UserId              int64
	SessionId           int64
	PageId              int64
	UserCreated         time.Time
	UserAppKey          string
	UserDisplayName     string
	UserEmail           string
	EventStart          time.Time
	EventType           string
	EventCustomName     string
	EventTargetText     string
	EventTargetSelector string
	EventModFrustrated  int64
	EventModDead        int64
	EventModError       int64
	EventModSuspicious  int64
	SessionStart        time.Time
	PageStart           time.Time
	PageDuration        int64
	PageActiveDuration  int64
	PageUrl             string
	PageRefererUrl      string
	PageIp              string
	PageLatLong         string
	PageUserAgent       string
	PageBrowser         string
	PageDevice          string
	PagePlatform        string
	PageOperatingSystem string
	PageScreenWidth     int64
	PageScreenHeight    int64
	PageViewportWidth   int64
	PageViewportHeight  int64
	PageNumInfos        int64
	PageNumWarnings     int64
	PageNumErrors       int64
	PageClusterId       int64
	LoadDomContentTime  int64
	LoadEventTime       int64
	LoadFirstPaintTime  int64
	CustomVars          string
}

type MobileFields struct {
	BaseExportFields
	AppName        string
	AppPackageName string
}

// syncTable represents all the fields that should appear in the table used to track which bundles have been synced.
type syncTable struct {
	ID            int64
	Processed     time.Time
	BundleEndTime time.Time
}

// WarehouseField contains metadata for a field/column in the warehouse.
type WarehouseField struct {
	// The name of the field as it exists in the database
	DBName string
	// The name of the field from FullStory
	FullStoryFieldName string
	FieldType          reflect.Type
	DBType             string
}

// BundleField contains metadata for an attribute on an event object in an export bundle JSON document.
type BundleField struct {
	Name        string
	IsTime      bool
	IsCustomVar bool
}

func (f WarehouseField) String() string {
	return fmt.Sprintf("%s %s", f.DBName, f.DBType)
}

func (f WarehouseField) Equals(other WarehouseField) bool {
	return f.FullStoryFieldName == other.FullStoryFieldName &&
		f.FieldType == other.FieldType &&
		f.DBType == other.DBType &&
		f.DBName == other.DBName
}

type Schema []WarehouseField

func (s Schema) String() string {
	ss := make([]string, len(s))
	for i, f := range s {
		ss[i] = f.String()
	}
	return strings.Join(ss, ",")
}

func (s Schema) Equals(other Schema) bool {
	if len(s) != len(other) {
		return false
	}
	for i := range s {
		if !s[i].Equals(other[i]) {
			return false
		}
	}
	return true
}

func (s Schema) GetFieldForName(col string) WarehouseField {
	testCol := strings.ToLower(col)
	isPageAgent := false
	if testCol == "pageagent" {
		testCol = strings.ToLower("PageUserAgent")
		isPageAgent = true
	} else if testCol == "eventtargetselectortok" {
		return WarehouseField{
			DBName:             "EventTargetSelectorTok",
			FullStoryFieldName: "EventTargetSelectorTok",
			FieldType:          reflect.TypeOf(""),
		}
	}

	for _, field := range s {
		if strings.ToLower(field.DBName) == testCol {
			if isPageAgent {
				return WarehouseField{
					DBName:             "PageAgent",
					FullStoryFieldName: field.FullStoryFieldName,
					FieldType:          field.FieldType,
					DBType:             field.DBType,
				}
			}
			return field
		}
	}
	return WarehouseField{
		DBName: col,
	}
}

func IndexField(needle WarehouseField, haystack Schema) int {
	for i, elm := range haystack {
		if needle.FullStoryFieldName == elm.FullStoryFieldName {
			return i
		}
	}
	return -1
}

func (s Schema) ReconcileWithExisting(colNames []string) Schema {
	newSchema := make([]WarehouseField, 0, len(s))
	for _, colName := range colNames {
		newSchema = append(newSchema, s.GetFieldForName(colName))
	}
	newSchema = append(newSchema, s.GetMissingFieldsFor(newSchema)...)
	return newSchema
}

func (s Schema) GetMissingFieldsFor(b Schema) []WarehouseField {
	ret := make([]WarehouseField, 0, len(s))
	for _, field := range s {
		if idx := IndexField(field, b); idx == -1 {
			ret = append(ret, field)
		}
	}
	return ret
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

func MakeSchema(val interface{}) Schema {
	t := reflect.TypeOf(val)
	result := make(Schema, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		result[i] = WarehouseField{
			DBName:             t.Field(i).Name,
			FullStoryFieldName: t.Field(i).Name, // Default to the same name
			FieldType:          t.Field(i).Type,
		}
	}
	return result
}

func SyncTableSchema(ftm FieldTypeMapper) Schema {
	return structToSchema(syncTable{}, ftm)
}

func DefaultBundleColumns() []string {
	t := reflect.TypeOf(BundleEvent{})
	cols := make([]string, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		cols[i] = t.Field(i).Name
	}
	return cols
}

func structToSchema(i interface{}, ftm FieldTypeMapper) Schema {
	t := reflect.TypeOf(i)
	result := make(Schema, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		result[i] = WarehouseField{
			DBName: t.Field(i).Name,
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
