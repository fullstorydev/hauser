package warehouse

import (
	"reflect"
	"strings"
	"time"
)

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

// Mobile Apps fields will not be available for accounts that do not have
// the feature.
type MobileFields struct {
	AppName        string
	AppPackageName string
}

var wildcardFields = []string{
	"user_*",
	"evt_*",
	"page_*",
}

// syncTable represents all the fields that should appear in the table used to track which bundles have been synced.
type syncTable struct {
	ID            int64
	Processed     time.Time
	BundleEndTime time.Time
}

// WarehouseField contains metadata for a field/column in the warehouse.
type WarehouseField struct {
	// The name of the field as it exists in the database.
	// By default this will match the `FullStoryFieldName`, but may not match for certain fields
	// if they've been renamed in the export. (e.g. "PageAgent").
	// If the database contains columns that are not part of the FullStory export, this field will still be populated.
	DBName string

	// The name of the field from FullStory.
	// This can be empty if an existing database table contains columns that FullStory does not include
	// in the export. This value, if non-empty, is used as part of the `CreateExport` request in the
	// `DataExportClient`
	FullStoryFieldName string

	// FieldType should be used by each database implementation to specify the datatype
	// for this column. This is only used when creating or modifying a database's schema.
	// If the `FullStoryFieldName` is blank, then this should be nil.
	FieldType reflect.Type
}

func (f WarehouseField) IsTime() bool {
	return f.FieldType == reflect.TypeOf(time.Time{})
}

type Schema []WarehouseField

func (s Schema) Equals(other Schema) bool {
	if len(s) != len(other) {
		return false
	}
	for i := range s {
		if s[i] != other[i] {
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
		if !strings.EqualFold(field.DBName, other[i].DBName) {
			return false
		}
	}
	return true
}

var specialCasedFields = map[string]WarehouseField{
	"pageagent": {
		DBName:             "PageAgent",
		FullStoryFieldName: "PageUserAgent",
		FieldType:          reflect.TypeOf(BaseExportFields{}.PageUserAgent),
	},
	"eventtargetselectortok": {
		DBName:             "EventTargetSelectorTok",
		FullStoryFieldName: "EventTargetSelectorTok",
		FieldType:          reflect.TypeOf(""),
	},
}

// GetFieldForName takes an existing column name and returns the matching schema field.
// It performs some conversions between the legacy field names and the new field names.
func (s Schema) GetFieldForName(col string) WarehouseField {
	if specialCasedField, ok := specialCasedFields[strings.ToLower(col)]; ok {
		return specialCasedField
	}
	for _, field := range s {
		if strings.EqualFold(field.DBName, col) {
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
		} else if field.FullStoryFieldName != "" {
			// Only add non-empty fields since the database can have existing columns which
			// are not part of the export.
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

// ReconcileWithExisting returns a new schema that is compatible with the provided column names.
// If exported fields are missing from the column list, they are appended to the end.
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

func MakeSchema(vals ...interface{}) Schema {
	var result Schema
	for _, val := range vals {
		t := reflect.TypeOf(val)
		for i := 0; i < t.NumField(); i++ {
			result = append(result, WarehouseField{
				DBName:             t.Field(i).Name,
				FullStoryFieldName: t.Field(i).Name, // Default to the same name
				FieldType:          t.Field(i).Type,
			})
		}
	}
	return result
}
