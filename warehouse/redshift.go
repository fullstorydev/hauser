package warehouse

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/s3"
	"github.com/lib/pq"
	"github.com/nishanths/fullstory"

	"github.com/fullstorydev/hauser/config"
)

type Redshift struct {
	conn         *sql.DB
	conf         *config.Config
	exportSchema Schema
	syncSchema   Schema
}

var (
	RedshiftTypeMap = FieldTypeMapper{
		"int64":     "BIGINT",
		"string":    "VARCHAR(max)",
		"time.Time": "TIMESTAMP",
	}
	beginningOfTime = time.Date(2015, 01, 01, 0, 0, 0, 0, time.UTC)
)

func NewRedshift(c *config.Config) *Redshift {
	if c.S3.S3Only {
		log.Printf("Config flag S3Only is on, data will not be loaded to Redshift")
	}
	return &Redshift{
		conf:         c,
		exportSchema: ExportTableSchema(RedshiftTypeMap),
		syncSchema:   SyncTableSchema(RedshiftTypeMap),
	}
}

func (rs *Redshift) getExportTableName(fullyQualifiedName bool) string {
	if rs.conf.Redshift.DatabaseSchema == "search_path" || !fullyQualifiedName {
		return rs.conf.Redshift.ExportTable
	}
	return fmt.Sprintf("%s.%s", rs.conf.Redshift.DatabaseSchema, rs.conf.Redshift.ExportTable)
}

func (rs *Redshift) getSyncTableName(fullyQualifiedName bool) string {
	if rs.conf.Redshift.DatabaseSchema == "search_path" || !fullyQualifiedName {
		return rs.conf.Redshift.SyncTable
	}
	return fmt.Sprintf("%s.%s", rs.conf.Redshift.DatabaseSchema, rs.conf.Redshift.SyncTable)
}

func (rs *Redshift) getSchemaParameter() string {
	// the built-in current_schema() function will walk the Postgres search_path to get a schema name
	// more info: https://www.postgresql.org/docs/9.4/functions-info.html
	if rs.conf.Redshift.DatabaseSchema == "search_path" {
		return "current_schema()"
	}

	return fmt.Sprintf("'%s'", rs.conf.Redshift.DatabaseSchema)
}

func (rs *Redshift) validateSchemaConfig() error {
	if rs.conf.Redshift.DatabaseSchema == "" {
		return errors.New("DatabaseSchema definition missing from Redshift configuration. More information: https://github.com/fullstorydev/hauser/blob/master/Redshift.md#database-schema-configuration")
	}
	return nil
}

// GetExportTableColumns returns all the columns of the export table.
// It opens a connection and calls getTableColumns
func (rs *Redshift) GetExportTableColumns() []string {
	var err error
	rs.conn, err = rs.MakeRedshiftConnection()
	if err != nil {
		log.Fatal(err)
	}
	defer rs.conn.Close()

	return rs.getTableColumns(rs.getExportTableName(false))
}

func (rs *Redshift) ValueToString(val interface{}, isTime bool) string {
	s := fmt.Sprintf("%v", val)
	if isTime {
		t, _ := time.Parse(time.RFC3339Nano, s)
		return t.String()
	}

	s = strings.Replace(s, "\n", " ", -1)
	s = strings.Replace(s, "\r", " ", -1)
	s = strings.Replace(s, "\x00", "", -1)

	if len(s) >= rs.conf.Redshift.VarCharMax {
		s = s[:rs.conf.Redshift.VarCharMax-1]
	}
	return s
}

func (rs *Redshift) MakeRedshiftConnection() (*sql.DB, error) {
	if err := rs.validateSchemaConfig(); err != nil {
		log.Fatal(err)
	}
	url := fmt.Sprintf("user=%v password=%v host=%v port=%v dbname=%v",
		rs.conf.Redshift.User,
		rs.conf.Redshift.Password,
		rs.conf.Redshift.Host,
		rs.conf.Redshift.Port,
		rs.conf.Redshift.DB)

	var err error
	var db *sql.DB
	if db, err = sql.Open("postgres", url); err != nil {
		return nil, fmt.Errorf("redshift connect error : (%v)", err)
	}

	if err = db.Ping(); err != nil {
		return nil, fmt.Errorf("redshift ping error : (%v)", err)
	}
	return db, nil
}

func (rs *Redshift) UploadFile(name string) (string, error) {
	file, err := os.Open(name)
	if err != nil {
		return "", err
	}

	sess := session.Must(session.NewSession())
	svc := s3.New(sess, aws.NewConfig().WithRegion(rs.conf.S3.Region))

	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, rs.conf.S3.Timeout.Duration)
	defer cancelFn()

	_, objName := filepath.Split(name)

	bucketName, key := getBucketAndKey(rs.conf.S3.Bucket, objName)

	_, err = svc.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(bucketName),
		Key:    aws.String(key),
		Body:   file,
	})

	s3path := fmt.Sprintf("s3://%s/%s", bucketName, key)
	return s3path, err
}

func getBucketAndKey(bucketConfig, objName string) (string, string) {
	bucketParts := strings.Split(bucketConfig, "/")
	bucketName := bucketParts[0]
	keyPath := strings.Trim(strings.Join(bucketParts[1:], "/"), "/")
	key := strings.Trim(fmt.Sprintf("%s/%s", keyPath, objName), "/")

	return bucketName, key
}

func (rs *Redshift) DeleteFile(s3obj string) {
	sess := session.Must(session.NewSession())
	svc := s3.New(sess, aws.NewConfig().WithRegion(rs.conf.S3.Region))

	ctx := context.Background()
	ctx, cancelFn := context.WithTimeout(ctx, rs.conf.S3.Timeout.Duration)
	defer cancelFn()

	_, objName := filepath.Split(s3obj)
	_, err := svc.DeleteObjectWithContext(ctx, &s3.DeleteObjectInput{
		Bucket: aws.String(rs.conf.S3.Bucket),
		Key:    aws.String(objName),
	})

	if err != nil {
		log.Printf("failed to delete S3 object %s: %s", s3obj, err)
		// just return - object will remain in S3
	}
}

func (rs *Redshift) LoadToWarehouse(s3obj string, _ ...fullstory.ExportMeta) error {
	var err error
	rs.conn, err = rs.MakeRedshiftConnection()
	if err != nil {
		return err
	}
	defer rs.conn.Close()

	if err = rs.CopyInData(s3obj); err != nil {
		return err
	}

	return nil
}

// EnsureCompatibleExportTable makes sure the export table has all the hauser schema columns
func (rs *Redshift) EnsureCompatibleExportTable() error {
	var err error
	rs.conn, err = rs.MakeRedshiftConnection()
	if err != nil {
		return err
	}
	defer rs.conn.Close()

	if !rs.DoesTableExist(rs.getExportTableName(false)) {
		// if the export table does not exist we create one with all the columns we expect!
		log.Printf("Export table %s does not exist! Creating one!", rs.getExportTableName(true))
		if err = rs.CreateExportTable(); err != nil {
			return err
		}
		return nil
	}

	// make sure all the columns in the csv export exist in the Export table
	exportTableColumns := rs.getTableColumns(rs.getExportTableName(false))
	missingFields := rs.getMissingFields(rs.exportSchema, exportTableColumns)

	// If some fields are missing from the fsexport table, either we added new fields
	// or existing expected columns were deleted by the user we add the relevant columns.
	// Alter the table and add the missing columns.
	if len(missingFields) > 0 {
		log.Printf("Found %d missing fields. Adding columns for these fields.", len(missingFields))
		for _, f := range missingFields {
			// Redshift only allows addition of one column at a time, hence the the alter statements in a loop yuck
			alterStmt := fmt.Sprintf("ALTER TABLE %s ADD COLUMN %s %s;", rs.getExportTableName(true), f.Name, f.DBType)
			if _, err = rs.conn.Exec(alterStmt); err != nil {
				return err
			}
		}
	}
	return nil
}

// CopyInData copies data from the given s3File to the export table
func (rs *Redshift) CopyInData(s3file string) error {
	copyStatement := fmt.Sprintf("COPY %s FROM '%s' CREDENTIALS '%s' DELIMITER ',' REGION '%s' FORMAT AS CSV ACCEPTINVCHARS;",
		rs.getExportTableName(true), s3file, rs.conf.Redshift.Credentials, rs.conf.S3.Region)
	_, err := rs.conn.Exec(copyStatement)
	return err
}

// CreateExportTable creates an export table with the hauser export table schema
func (rs *Redshift) CreateExportTable() error {
	log.Printf("Creating table %s", rs.getExportTableName(true))

	stmt := fmt.Sprintf("create table %s(%s);", rs.getExportTableName(true), rs.exportSchema.String())
	_, err := rs.conn.Exec(stmt)
	return err
}

// CreateSyncTable creates a sync table with the hauser sync table schema
func (rs *Redshift) CreateSyncTable() error {
	log.Printf("Creating table %s", rs.getSyncTableName(true))

	stmt := fmt.Sprintf("create table %s(%s);", rs.getSyncTableName(true), rs.syncSchema.String())
	_, err := rs.conn.Exec(stmt)
	return err
}

func (rs *Redshift) SaveSyncPoints(bundles ...fullstory.ExportMeta) error {
	var err error
	rs.conn, err = rs.MakeRedshiftConnection()
	if err != nil {
		log.Printf("Couldn't connect to DB: %s", err)
		return err
	}
	defer rs.conn.Close()

	for _, e := range bundles {
		insert := fmt.Sprintf("insert into %s values (%d, '%s', '%s')",
			rs.getSyncTableName(true), e.ID, time.Now().Format(time.RFC3339), e.Stop.Format(time.RFC3339))
		if _, err := rs.conn.Exec(insert); err != nil {
			return err
		}
	}

	return nil
}

func (rs *Redshift) DeleteExportRecordsAfter(end time.Time) error {
	stmt := fmt.Sprintf("DELETE FROM %s where EventStart > '%s';",
		rs.getExportTableName(true), end.Format(time.RFC3339))
	_, err := rs.conn.Exec(stmt)
	if err != nil {
		log.Printf("failed to delete from %s: %s", rs.getExportTableName(true), err)
		return err
	}

	return nil
}

func (rs *Redshift) LastSyncPoint() (time.Time, error) {
	t := beginningOfTime
	var err error
	rs.conn, err = rs.MakeRedshiftConnection()
	if err != nil {
		log.Printf("Couldn't connect to DB: %s", err)
		return t, err
	}
	defer rs.conn.Close()

	if rs.DoesTableExist(rs.getSyncTableName(false)) {
		var syncTime pq.NullTime
		q := fmt.Sprintf("SELECT max(BundleEndTime) FROM %s;", rs.getSyncTableName(true))
		if err := rs.conn.QueryRow(q).Scan(&syncTime); err != nil {
			log.Printf("Couldn't get max(BundleEndTime): %s", err)
			return t, err
		}
		if syncTime.Valid {
			t = syncTime.Time
		}

		if err := rs.RemoveOrphanedRecords(syncTime); err != nil {
			return t, err
		}

	} else {
		if err := rs.CreateSyncTable(); err != nil {
			log.Printf("Couldn't create sync table: %s", err)
			return t, err
		}
	}
	return t, nil
}

func (rs *Redshift) RemoveOrphanedRecords(lastSync pq.NullTime) error {
	if rs.conf.S3.S3Only {
		// no need to check for orphaned records, as we're not loading to the export table
		return nil
	}

	if !rs.DoesTableExist(rs.getExportTableName(false)) {
		if err := rs.CreateExportTable(); err != nil {
			log.Printf("Couldn't create export table: %s", err)
			return err
		}
	}

	// Find the time of the latest export record...if it's after
	// the time in the sync table, then there must have been a failure
	// after some records have been loaded, but before the sync record
	// was written. Use this as the latest sync time, and don't load
	// any records before this point to prevent duplication
	var exportTime pq.NullTime
	q := fmt.Sprintf("SELECT max(EventStart) FROM %s;", rs.getExportTableName(true))
	if err := rs.conn.QueryRow(q).Scan(&exportTime); err != nil {
		log.Printf("Couldn't get max(EventStart): %s", err)
		return err
	}
	if exportTime.Valid && exportTime.Time.After(lastSync.Time) {
		log.Printf("Export record timestamp after sync time (%s vs %s); cleaning",
			exportTime.Time, lastSync.Time)
		rs.DeleteExportRecordsAfter(lastSync.Time)
	}

	return nil
}

// DoesTableExist checks if a table with a given name exists
func (rs *Redshift) DoesTableExist(name string) bool {
	log.Printf("Checking if table %s exists", name)

	var exists int
	query := fmt.Sprintf("SELECT count(*) FROM information_schema.tables WHERE table_schema = %s AND table_name = $1;", rs.getSchemaParameter())
	err := rs.conn.QueryRow(query, name).Scan(&exists)
	if err != nil {
		// something is horribly wrong...just give up
		log.Fatal(err)
	}
	return (exists != 0)
}

func (rs *Redshift) getTableColumns(name string) []string {
	log.Printf("Fetching columns for table %s", name)
	ctx := context.Background()
	query := fmt.Sprintf("SELECT column_name FROM information_schema.columns WHERE table_schema = %s AND table_name  = $1;", rs.getSchemaParameter())
	rows, err := rs.conn.QueryContext(ctx, query, name)
	if err != nil {
		log.Fatal(err)
	}
	var columns []string

	defer rows.Close()
	for rows.Next() {
		var column string
		if err = rows.Scan(&column); err != nil {
			log.Fatal(err)
		}
		columns = append(columns, column)
	}

	// get any error encountered during iteration
	if err = rows.Err(); err != nil {
		log.Fatal(err)
	}
	return columns
}

func (rs *Redshift) getMissingFields(schema Schema, tableColumns []string) []WarehouseField {
	existingColumns := make(map[string]struct{})
	for _, column := range tableColumns {
		// Redshift columns are case insensitive
		existingColumns[strings.ToLower(column)] = struct{}{}
	}

	var missingFields []WarehouseField
	for _, f := range schema {
		if _, ok := existingColumns[strings.ToLower(f.Name)]; !ok {
			missingFields = append(missingFields, f)
		}
	}

	return missingFields
}

func (rs *Redshift) GetUploadFailedMsg(filename string, err error) string {
	return fmt.Sprintf("Failed to upload file %s to s3: %s", filename, err)
}

func (rs *Redshift) IsUploadOnly() bool {
	return rs.conf.S3.S3Only
}
