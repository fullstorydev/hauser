package warehouse

import (
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

var wildcardFields = []string{
	"user_*",
	"evt_*",
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
}

// BundleField contains metadata for an attribute on an event object in an export bundle JSON document.
type BundleField struct {
	Name        string
	IsTime      bool
	IsCustomVar bool
}

func (f WarehouseField) Equals(other WarehouseField) bool {
	return f.FullStoryFieldName == other.FullStoryFieldName &&
		f.FieldType == other.FieldType &&
		f.DBName == other.DBName
}

type Schema []WarehouseField

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

func (s Schema) IsCompatibleWith(other Schema) bool {
	if len(s) > len(other) {
		return false
	}
	for i, field := range s {
		if strings.ToLower(field.DBName) != strings.ToLower(other[i].DBName) {
			return false
		}
	}
	return true
}

// GetFieldForName takes an existing column name and returns the matching schema field.
// It performs some conversions between the legacy field names and the new field names.
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
				}
			}
			return field
		}
	}
	return WarehouseField{
		DBName: col,
	}
}

func (s Schema) GetFullStoryFields() []string {
	fsFields := make([]string, 0, len(s))
	for _, field := range s {
		if field.FullStoryFieldName == "CustomVars" {
			// We have to special case custom vars since they are combined into a single column
			// Add the wildcards for each type of exportable custom var.
			fsFields = append(fsFields, wildcardFields...)
		} else {
			fsFields = append(fsFields, field.FullStoryFieldName)
		}
	}
	return fsFields
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
