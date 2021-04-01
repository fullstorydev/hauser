package warehouse

import (
	"database/sql"
	"errors"
	"fmt"
	"log"
	"reflect"
	"time"

	"github.com/fullstorydev/hauser/config"
	_ "github.com/lib/pq"
)

type Redshift struct {
	DBMixin
	conf       *config.RedshiftConfig
	syncSchema Schema
}

var (
	redshiftSchemaMap = map[reflect.Type]string{
		reflect.TypeOf(int64(0)):    "BIGINT",
		reflect.TypeOf(""):          "VARCHAR(max)",
		reflect.TypeOf(time.Time{}): "TIMESTAMP",
	}
)

var _ Database = (*Redshift)(nil)

func NewRedshift(c *config.RedshiftConfig) *Redshift {
	return &Redshift{
		DBMixin: DBMixin{
			exportTable: c.ExportTable,
			syncTable:   c.SyncTable,
			schema:      c.DatabaseSchema,
			typeMap:     redshiftSchemaMap,
		},
		conf:       c,
		syncSchema: MakeSchema(syncTable{}),
	}
}

func (rs *Redshift) validateSchemaConfig() error {
	if rs.conf.DatabaseSchema == "" {
		return errors.New("DatabaseSchema definition missing from Redshift configuration. More information: https://github.com/fullstorydev/hauser/blob/master/Redshift.md#database-schema-configuration")
	}
	return nil
}

func (rs *Redshift) ValueToString(val interface{}, isTime bool) string {
	s := rs.DBMixin.ValueToString(val, isTime)
	if len(s) >= rs.conf.VarCharMax {
		s = s[:rs.conf.VarCharMax-1]
	}
	return s
}

func (rs *Redshift) ensureConnection() error {
	if rs.db == nil {

		if err := rs.validateSchemaConfig(); err != nil {
			log.Fatal(err)
		}
		url := fmt.Sprintf("user=%v password=%v host=%v port=%v dbname=%v",
			rs.conf.User,
			rs.conf.Password,
			rs.conf.Host,
			rs.conf.Port,
			rs.conf.DB)

		var err error
		if rs.db, err = sql.Open("postgres", url); err != nil {
			return fmt.Errorf("redshift connect error : (%v)", err)
		}
	}
	return rs.db.Ping()
}

func (rs *Redshift) LoadToWarehouse(s3obj string, _ time.Time) error {
	if err := rs.ensureConnection(); err != nil {
		return err
	}

	if err := rs.CopyInData(s3obj); err != nil {
		return err
	}
	return nil
}

func (rs *Redshift) InitExportTable(schema Schema) (bool, error) {
	if err := rs.ensureConnection(); err != nil {
		return false, err
	}
	return rs.DBMixin.InitExportTable(schema)
}

func (rs *Redshift) ApplyExportSchema(newSchema Schema) error {
	return rs.applySchema(rs.conf.ExportTable, newSchema, redshiftSchemaMap)
}

// CopyInData copies data from the given s3File to the export table
func (rs *Redshift) CopyInData(s3file string) error {
	copyStatement := fmt.Sprintf("COPY %s FROM '%s' CREDENTIALS '%s' DELIMITER ',' REGION '%s' FORMAT AS CSV IGNOREHEADER 1 ACCEPTINVCHARS;",
		formatTableName(rs.schema, rs.exportTable), s3file, rs.conf.Credentials, rs.conf.S3Region)
	_, err := rs.db.Exec(copyStatement)
	return err
}
