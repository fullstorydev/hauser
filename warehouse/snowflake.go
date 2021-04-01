package warehouse

import (
	"database/sql"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"

	"github.com/fullstorydev/hauser/config"
	"github.com/snowflakedb/gosnowflake"
	_ "github.com/snowflakedb/gosnowflake"
)

type Snowflake struct {
	DBMixin
	conf       *config.SnowflakeConfig
	syncSchema Schema
}

var (
	snowflakeSchemaMap = map[reflect.Type]string{
		reflect.TypeOf(int64(0)):    "BIGINT",
		reflect.TypeOf(""):          "VARCHAR",
		reflect.TypeOf(time.Time{}): "TIMESTAMP",
	}
)

var _ Database = (*Snowflake)(nil)

//func PrintSchema() {
//	schema := MakeSchema(BaseExportFields{})
//	rs := makeDbSchema(schema, snowflakeSchemaMap)
//	fmt.Printf("%s", rs)
//}

func NewSnowflake(c *config.SnowflakeConfig) *Snowflake {
	return &Snowflake{
		DBMixin: DBMixin{
			exportTable: c.ExportTable,
			syncTable:   c.SyncTable,
			schema:      c.Schema,
			typeMap:     snowflakeSchemaMap,
		},
		conf:       c,
		syncSchema: MakeSchema(syncTable{}),
	}
}

func (sf *Snowflake) ensureConnection() error {
	if sf.db == nil {
		dsn, err := gosnowflake.DSN(&gosnowflake.Config{
			Account:   sf.conf.Account,
			User:      sf.conf.User,
			Password:  sf.conf.Password,
			Database:  sf.conf.Database,
			Schema:    sf.conf.Schema,
			Role:      sf.conf.Role,
			Warehouse: sf.conf.Warehouse,
		})
		if err != nil {
			return err
		}
		sf.db, err = sql.Open("snowflake", dsn)
		if err != nil {
			return err
		}
	}
	return sf.db.Ping()
}

func (sf *Snowflake) LoadToWarehouse(storageRef string, _ time.Time) error {
	stmt := fmt.Sprintf(`
copy into %s from @%s
pattern = '%s'
file_format = (type = csv field_delimiter = ',' skip_hdeader = 1 field_optionally_enclosed_by = '"');
`,
		formatTableName(sf.schema, sf.exportTable), sf.conf.StageName, getFilePattenFromRef(storageRef))
	log.Printf("Executing SQL: %s", stmt)
	if _, err := sf.db.Exec(stmt); err != nil {
		return err
	}
	return nil
}

func (sf *Snowflake) InitExportTable(s Schema) (bool, error) {
	if err := sf.ensureConnection(); err != nil {
		return false, err
	}

	//if err := sf.validateFileFormat(); err != nil {
	//	return false, err
	//}

	if err := sf.validateStage(); err != nil {
		return false, err
	}

	return sf.DBMixin.InitExportTable(s)
}

//func (sf *Snowflake) ValueToString(val interface{}, isTime bool) string {
//	s := fmt.Sprintf("%v", val)
//	if isTime {
//		t, _ := time.Parse(time.RFC3339Nano, s)
//		return t.String()
//	}
//
//	s = strings.Replace(s, "\n", " ", -1)
//	s = strings.Replace(s, "\r", " ", -1)
//	s = strings.Replace(s, "\x00", "", -1)
//}

func (sf *Snowflake) validateFileFormat() error {
	// Verify that the file format exists and has the right config.
	//      type: 'csv'
	//		delimiter: ','
	// 		skip_header: 1
	//		field_optionally_enclosed_by = '"'
	// fmt.Sprintf("desc file format %s;", sf.conf.FileFormatName)
	rows, err := sf.db.Query(fmt.Sprintf("desc file format %s;", sf.conf.FileFormatName))
	if err != nil {
		return err
	}
	defer rows.Close()

	for rows.Next() {
		var property string
		var typ string
		var value string
		var dflt string
		if err := rows.Scan(&property, &typ, &value, &dflt); err != nil {
			return err
		}
		switch strings.ToLower(property) {
		case "type":
			if strings.ToLower(value) != "csv" {
				return fmt.Errorf("invalid format type: %s", value)
			}
		case "field_delimiter":
			if value != "," {
				return fmt.Errorf("invalid delimiter: %s", value)
			}
		case "record_delimiter":
			if value != "\\n" {
				return fmt.Errorf("invalid record delimiter: %s", value)
			}
		case "skip_header":
			if value != "1" {
				return fmt.Errorf("invalid skip_header value: %s", value)
			}
		case "field_optionally_enclosed_by":
			if value != "\\\"" {
				return fmt.Errorf("invalid field_optionally_enclosed_by value: %s", value)
			}
		}
	}
	return rows.Err()
}

func (sf *Snowflake) validateStage() error {
	// Verify that the stage exists and has the right settings
	// fmt.Sprintf("desc stage %s;", sf.conf.StageName)
	// 		stage_file_format == sf.conf.FileFormatName
	//		stage_location.contains(sf.conf.StageLocation)

	rows, err := sf.db.Query(fmt.Sprintf("desc stage %s;", sf.conf.StageName))
	if err != nil {
		return err
	}
	rows.Close()

	var formatName string
	if err := sf.db.
		QueryRow("select \"property_value\" from table(result_scan(last_query_id())) where \"property\" = 'FORMAT_NAME'").
		Scan(&formatName); err != nil {
		return err
	}
	if formatName != sf.conf.FileFormatName {
		return fmt.Errorf("unexpected format for stage: %s", formatName)
	}
	return nil
}

var storageSchemes = []string{"gs://", "s3://"}

func getFilePattenFromRef(storageRef string) string {
	basePattern := storageRef
	for _, scheme := range storageSchemes {
		if strings.Index(storageRef, scheme) >= 0 {
			// pull off the scheme and  bucket name
			splits := strings.SplitAfterN(strings.TrimPrefix(storageRef, scheme), "/", 2)
			if len(splits) == 2 {
				basePattern = splits[1]
			} else {
				basePattern = splits[0]
			}

		}
	}
	return fmt.Sprintf(".*%s", basePattern)
}
