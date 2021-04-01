package warehouse

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"reflect"
	"strings"
	"time"
)

const inferSchemaValue = "search_path"

var (
	defaultTypeMap = map[reflect.Type]string{
		reflect.TypeOf(int64(0)):    "BIGINT",
		reflect.TypeOf(""):          "VARCHAR",
		reflect.TypeOf(time.Time{}): "TIMESTAMP",
	}
)

type columnConfig struct {
	DBName string
	DBType string
}

type dbSchema []columnConfig

func (c dbSchema) String() string {
	ss := make([]string, len(c))
	for i, f := range c {
		ss[i] = fmt.Sprintf("%s %s", f.DBName, f.DBType)
	}
	return strings.Join(ss, ",")
}

func makeDbSchema(s Schema, typeMap map[reflect.Type]string) dbSchema {
	columns := make([]columnConfig, len(s))
	for i, field := range s {
		dbType, ok := typeMap[field.FieldType]
		if !ok {
			panic(fmt.Sprintf("field %s does not have a mapping to a database type", field.DBName))
		}
		columns[i] = columnConfig{
			DBName: field.DBName,
			DBType: dbType,
		}
	}
	return columns
}

// If not inferring the schema, we need to quote the value
func schemaSelectValue(schema string) string {
	if schema == inferSchemaValue {
		return "current_schema()"
	}
	return fmt.Sprintf("'%s'", schema)
}

func formatTableName(schema, table string) string {
	if schema == inferSchemaValue {
		return table
	}
	return fmt.Sprintf("%s.%s", schema, table)
}

type DBMixin struct {
	db          *sql.DB
	schema      string
	syncTable   string
	exportTable string
	typeMap     map[reflect.Type]string
}

var _ Database = (*DBMixin)(nil)

func (m *DBMixin) LastSyncPoint(_ context.Context) (time.Time, error) {
	t := time.Time{}
	if exists, err := m.tableExists(m.syncTable); err != nil {
		log.Fatalf("failed to check for export table existence: %s", err)

	} else if exists {
		var syncTime sql.NullTime
		q := fmt.Sprintf("SELECT max(BundleEndTime) FROM %s;", formatTableName(m.schema, m.syncTable))
		if err := m.db.QueryRow(q).Scan(&syncTime); err != nil {
			log.Printf("Couldn't get max(BundleEndTime): %s", err)
			return t, err
		}
		if syncTime.Valid {
			t = syncTime.Time
		}

		if err := m.removeOrphanedRecords(syncTime); err != nil {
			return t, err
		}

	} else {
		if err := m.createTable(m.syncTable, makeDbSchema(MakeSchema(syncTable{}), m.typeMap)); err != nil {
			log.Printf("Couldn't create sync table: %s", err)
			return t, err
		}
	}
	return t, nil
}

func (m *DBMixin) SaveSyncPoint(_ context.Context, endTime time.Time) error {
	return m.insertSyncRow(endTime)
}

func (m *DBMixin) LoadToWarehouse(_ string, _ time.Time) error {
	panic("database missing implementation for 'LoadToWarehouse'")
}

func (m *DBMixin) ValueToString(val interface{}, isTime bool) string {
	s := fmt.Sprintf("%v", val)
	if isTime {
		t, _ := time.Parse(time.RFC3339Nano, s)
		return t.Format(RFC3339Micro)
	}

	s = strings.Replace(s, "\n", " ", -1)
	s = strings.Replace(s, "\r", " ", -1)
	s = strings.Replace(s, "\x00", "", -1)
	return s
}

func (m *DBMixin) GetExportTableColumns() []string {
	cols, err := m.getTableColumns(m.exportTable)
	if err != nil {
		log.Fatalf("failed to get export table columns: %s", err)
	}
	return cols
}

func (m *DBMixin) InitExportTable(schema Schema) (bool, error) {
	if exists, err := m.tableExists(m.exportTable); err != nil {
		log.Fatalf("failed to check for export table existence: %s", err)
	} else if !exists {
		// if the export table does not exist we create one with all the columns we expect!
		log.Printf("Export table %s does not exist! Creating one!", formatTableName(m.schema, m.exportTable))
		if err = m.createTable(m.exportTable, makeDbSchema(schema, m.typeMap)); err != nil {
			return false, err
		}
		return true, nil
	}
	return false, nil
}

func (m *DBMixin) ApplyExportSchema(newSchema Schema) error {
	return m.applySchema(m.exportTable, newSchema, defaultTypeMap)
}

func (m *DBMixin) removeOrphanedRecords(lastSync sql.NullTime) error {
	if exists, err := m.tableExists(m.exportTable); err != nil {
		log.Fatalf("failed to check for export table existence: %s", err)
	} else if !exists {
		// This is okay, because the hauser process will ensure that the table exists.
		return nil
	}

	// Find the time of the latest export record...if it's after
	// the time in the sync table, then there must have been a failure
	// after some records have been loaded, but before the sync record
	// was written. Use this as the latest sync time, and don't load
	// any records before this point to prevent duplication
	var exportTime sql.NullTime
	q := fmt.Sprintf("SELECT max(EventStart) FROM %s;", formatTableName(m.schema, m.exportTable))
	if err := m.db.QueryRow(q).Scan(&exportTime); err != nil {
		log.Printf("Couldn't get max(EventStart): %s", err)
		return err
	}
	if exportTime.Valid && exportTime.Time.After(lastSync.Time) {
		log.Printf("Export record timestamp after sync time (%s vs %s); cleaning",
			exportTime.Time, lastSync.Time)
		return m.deleteExportRecordsAfter(lastSync.Time)
	}
	return nil
}

func (m *DBMixin) tableExists(table string) (bool, error) {
	query := fmt.Sprintf("SELECT count(*) FROM information_schema.tables WHERE table_schema = %s AND table_name = '%s'", schemaSelectValue(m.schema), table)
	log.Printf("Checking existence: %s", query)
	var exists int
	if err := m.db.QueryRow(query).Scan(&exists); err != nil {
		return false, err
	}
	return exists != 0, nil
}

func (m *DBMixin) createTable(table string, columns dbSchema) error {
	stmt := fmt.Sprintf("create table %s(%s)", formatTableName(m.schema, table), columns)
	log.Printf("Creating table: %s", stmt)
	_, err := m.db.Exec(stmt)
	return err
}

func (m *DBMixin) addColumn(table string, config columnConfig) error {
	stmt := fmt.Sprintf("alter table %s add column %s %s", formatTableName(m.schema, table), config.DBName, config.DBType)
	_, err := m.db.Exec(stmt)
	return err
}

func (m *DBMixin) applySchema(table string, newSchema Schema, typeMap map[reflect.Type]string) error {
	existingColumns, err := m.getTableColumns(table)
	if err != nil {
		return err
	}
	missingFields, err := getColumnsToAdd(newSchema, existingColumns, typeMap)
	if err != nil {
		return err
	}
	if len(missingFields) > 0 {
		log.Printf("Found %d missing fields. Adding columns for these fields.", len(missingFields))
		for _, f := range missingFields {
			if err := m.addColumn(table, f); err != nil {
				return err
			}
		}
	}
	return nil
}

func getColumnsToAdd(s Schema, existing []string, typeMap map[reflect.Type]string) ([]columnConfig, error) {
	if len(s) < len(existing) {
		return nil, fmt.Errorf("incompatible schema: have %v, got %v", existing, s)
	}
	missing := make([]columnConfig, len(s)-len(existing))
	for i := 0; i < len(missing); i++ {
		field := s[len(existing)+i]
		missing[i] = columnConfig{
			DBName: field.DBName,
			DBType: typeMap[field.FieldType],
		}
	}
	return missing, nil
}

func (m *DBMixin) getTableColumns(table string) ([]string, error) {
	query := fmt.Sprintf("SELECT column_name FROM information_schema.columns WHERE table_schema = %s AND table_name = '%s' order by ordinal_position;", schemaSelectValue(m.schema), table)
	rows, err := m.db.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var columns []string
	for rows.Next() {
		var column string
		if err := rows.Scan(&column); err != nil {
			return nil, err
		}
		columns = append(columns, column)
	}
	return columns, rows.Err()
}

func (m *DBMixin) insertSyncRow(t time.Time) error {
	insert := fmt.Sprintf("insert into %s values (%d, '%s', '%s')",
		formatTableName(m.schema, m.syncTable), -1, time.Now().UTC().Format(time.RFC3339), t.UTC().Format(time.RFC3339))
	_, err := m.db.Exec(insert)
	return err
}

func (m *DBMixin) deleteExportRecordsAfter(t time.Time) error {
	tName := formatTableName(m.schema, m.exportTable)
	stmt := fmt.Sprintf("DELETE FROM %s where EventStart > '%s';", tName, t.Format(time.RFC3339))
	_, err := m.db.Exec(stmt)
	if err != nil {
		return fmt.Errorf("failed to delete from %s: %s", tName, err)
	}
	return nil
}
