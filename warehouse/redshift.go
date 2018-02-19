package warehouse

import (
	"context"
	"database/sql"
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

func (rs *Redshift) MakeRedshfitConnection() (*sql.DB, error) {
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

func (rs *Redshift) MoveFiletoS3(name string) (string, error) {
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
	s3path := fmt.Sprintf("s3://%s/%s", rs.conf.S3.Bucket, objName)

	_, err = svc.PutObjectWithContext(ctx, &s3.PutObjectInput{
		Bucket: aws.String(rs.conf.S3.Bucket),
		Key:    aws.String(objName),
		Body:   file,
	})

	return s3path, err
}

func (rs *Redshift) RemoveS3Object(s3obj string) {
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

func (rs *Redshift) LoadToWarehouse(file string, _ ...fullstory.ExportMeta) error {
	var s3obj string
	var err error

	if s3obj, err = rs.MoveFiletoS3(file); err != nil {
		log.Printf("Failed to upload file %s to s3: %s", file, err)
		return err
	}

	if rs.conf.S3.S3Only {
		return nil
	}

	defer rs.RemoveS3Object(s3obj)

	rs.conn, err = rs.MakeRedshfitConnection()
	if err != nil {
		return err
	}
	defer rs.conn.Close()

	if !rs.DoesTableExist(rs.conf.Redshift.ExportTable) {
		if err := rs.CreateExportTable(); err != nil {
			return err
		}
	}

	if err := rs.CopyInData(s3obj); err != nil {
		return err
	}

	return nil
}

func (rs *Redshift) CopyInData(s3file string) error {
	log.Printf("Loading in data from %s", s3file)
	copy := fmt.Sprintf("copy %s from '%s' credentials '%s' delimiter ',' region '%s' format as csv ACCEPTINVCHARS;",
		rs.conf.Redshift.ExportTable, s3file, rs.conf.Redshift.Credentials, rs.conf.S3.Region)
	_, err := rs.conn.Exec(copy)
	return err
}

func (rs *Redshift) CreateExportTable() error {
	log.Printf("Creating table %s", rs.conf.Redshift.ExportTable)

	stmt := fmt.Sprintf("create table %s(%s);", rs.conf.Redshift.ExportTable, rs.exportSchema.String())
	_, err := rs.conn.Exec(stmt)
	return err
}

func (rs *Redshift) CreateSyncTable() error {
	log.Printf("Creating table %s", rs.conf.Redshift.SyncTable)

	stmt := fmt.Sprintf("create table %s(%s);", rs.conf.Redshift.SyncTable, rs.syncSchema.String())
	_, err := rs.conn.Exec(stmt)
	return err
}

func (rs *Redshift) SaveSyncPoints(bundles ...fullstory.ExportMeta) error {
	var err error
	rs.conn, err = rs.MakeRedshfitConnection()
	if err != nil {
		log.Printf("Couldn't connect to DB: %s", err)
		return err
	}
	defer rs.conn.Close()

	for _, e := range bundles {
		insert := fmt.Sprintf("insert into %s values (%d, '%s', '%s')",
			rs.conf.Redshift.SyncTable, e.ID, time.Now().Format(time.RFC3339), e.Stop.Format(time.RFC3339))
		if _, err := rs.conn.Exec(insert); err != nil {
			return err
		}
	}

	return nil
}

func (rs *Redshift) DeleteExportRecordsAfter(end time.Time) error {
	stmt := fmt.Sprintf("DELETE FROM %s where EventStart > '%s';",
		rs.conf.Redshift.ExportTable, end.Format(time.RFC3339))
	_, err := rs.conn.Exec(stmt)
	if err != nil {
		log.Printf("failed to delete from %s: %s", rs.conf.Redshift.ExportTable, err)
		return err
	}

	return nil
}

func (rs *Redshift) LastSyncPoint() (time.Time, error) {
	t := beginningOfTime
	var err error
	rs.conn, err = rs.MakeRedshfitConnection()
	if err != nil {
		log.Printf("Couldn't connect to DB: %s", err)
		return t, err
	}
	defer rs.conn.Close()

	if rs.DoesTableExist(rs.conf.Redshift.SyncTable) {
		var syncTime pq.NullTime
		q := fmt.Sprintf("SELECT max(BundleEndTime) FROM %s;", rs.conf.Redshift.SyncTable)
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

	if !rs.DoesTableExist(rs.conf.Redshift.ExportTable) {
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
	q := fmt.Sprintf("SELECT max(EventStart) FROM %s;", rs.conf.Redshift.ExportTable)
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

func (rs *Redshift) DoesTableExist(name string) bool {
	log.Printf("Checking if table %s exists", name)
	var exists int
	err := rs.conn.QueryRow("SELECT count(*) FROM pg_tables WHERE schemaname = 'public' AND tablename = $1;", name).Scan(&exists)
	if err != nil {
		// something is horribly wrong...just give up
		log.Fatal(err)
	}
	return (exists != 0)
}
