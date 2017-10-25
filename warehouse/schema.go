package warehouse

import (
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"
)

type bundleSchema struct {
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
	CustomVars             string
}

type syncTable struct {
	ID            int64
	Processed     time.Time
	BundleEndTime time.Time
}

type WarehouseField struct {
	Name        string
	DBType      string
}

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

func BundleSchema() []BundleField {
	t := reflect.TypeOf(bundleSchema{})
	result := make([]BundleField, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		result[i] = BundleField{
			Name:        t.Field(i).Name,
			IsTime:      t.Field(i).Type == reflect.TypeOf(time.Time{}),
			IsCustomVar: t.Field(i).Name == "CustomVars",
		}
	}
	return result
}

func ExportTableSchema(ftm FieldTypeMapper) Schema {
	// for now, the export table schema is an exact copy of the bundle schema
	return structToSchema(bundleSchema{}, ftm)
}

func SyncTableSchema(ftm FieldTypeMapper) Schema {
	return structToSchema(syncTable{}, ftm)
}

func structToSchema(i interface{}, ftm FieldTypeMapper) Schema {
	t := reflect.TypeOf(i)
	result := make(Schema, t.NumField())
	for i := 0; i < t.NumField(); i++ {
		result[i] = WarehouseField{
			Name:        t.Field(i).Name,
			DBType:      convertType(ftm, t.Field(i).Type),
		}
	}
	return result
}

func convertType(ftm FieldTypeMapper, t reflect.Type) string {
	dbtype, ok := ftm[t.String()]
	if !ok {
		log.Fatal("Type %s is not present in FieldTypeMapper", t)
	}
	return dbtype
}
