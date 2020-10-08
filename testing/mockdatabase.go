package testing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/fullstorydev/hauser/warehouse"
)

type MockDatabase struct {
	schema         warehouse.Schema
	initialColumns []string
	Initialized    bool
	Syncs          []time.Time
	LoadedFiles    []string
}

func (m *MockDatabase) InitExportTable(schema warehouse.Schema) (bool, error) {
	if len(m.schema) == 0 {
		// We're creating the table
		m.schema = schema
		m.Initialized = true
		return true, nil
	}
	return false, nil
}

func (m *MockDatabase) ApplyExportSchema(newSchema warehouse.Schema) error {
	if m.schema.IsCompatibleWith(newSchema) {
		m.schema = newSchema
		m.Initialized = true
		return nil
	}
	return errors.New(fmt.Sprintf("incompatible schema: have %v, got %v", m.schema, newSchema))
}

var _ warehouse.Database = (*MockDatabase)(nil)

func NewMockDatabase(existingColumns []string) *MockDatabase {
	defaultSchema := warehouse.MakeSchema(warehouse.BaseExportFields{})
	initialSchema := make(warehouse.Schema, len(existingColumns))
	for i, fieldName := range existingColumns {
		initialSchema[i] = defaultSchema.GetFieldForName(fieldName)
	}
	return &MockDatabase{
		schema:         initialSchema,
		initialColumns: existingColumns,
		Initialized:    false,
		Syncs:          nil,
		LoadedFiles:    nil,
	}
}

func (m *MockDatabase) LastSyncPoint(_ context.Context) (time.Time, error) {
	var max time.Time
	for i, s := range m.Syncs {
		if i == 0 || s.After(max) {
			max = s
		}
	}
	return max, nil
}

func (m *MockDatabase) SaveSyncPoint(_ context.Context, endTime time.Time) error {
	m.Syncs = append(m.Syncs, endTime)
	return nil
}

func (m *MockDatabase) LoadToWarehouse(filename string, _ time.Time) error {
	m.LoadedFiles = append(m.LoadedFiles, filename)
	return nil
}

func (m *MockDatabase) ValueToString(val interface{}, isTime bool) string {
	s := fmt.Sprintf("%v", val)
	if isTime {
		t, _ := time.Parse(time.RFC3339Nano, s)
		return t.Format(warehouse.RFC3339Micro)
	}
	return s
}

func (m *MockDatabase) GetExportTableColumns() []string {
	cols := make([]string, 0, len(m.schema))
	for _, f := range m.schema {
		cols = append(cols, f.DBName)
	}
	return cols
}
