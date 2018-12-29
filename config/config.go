package config

import (
	"io/ioutil"
	"time"
	"errors"

	"github.com/BurntSushi/toml"
)

type Config struct {
	Warehouse       string
	FsApiToken      string
	Backoff         duration
	BackoffStepsMax int
	CheckInterval   duration
	TmpDir          string
	ListExportLimit int
	GroupFilesByDay bool

	// for debug only; can point to localhost
	ExportURL string

	// aws: s3 + redshift
	S3       S3Config
	Redshift RedshiftConfig

	// gcloud: GCS + BigQuery
	GCS      GCSConfig
	BigQuery BigQueryConfig
}

type S3Config struct {
	Bucket  string
	Region  string
	Timeout duration
	S3Only  bool
}

type IRedshiftValidator interface {
	ValidateDatabaseSchema() error
}

type RedshiftConfigFields struct {
	Host           string
	Port           string
	DB             string
	User           string
	Password       string
	ExportTable    string
	SyncTable      string
	DatabaseSchema string
	Credentials    string
	VarCharMax     int
}

type RedshiftConfig struct {
	RedshiftConfigFields
	Validator IRedshiftValidator
}

type RedshiftValidator struct {
	RedshiftConfigFields
 }

func (v RedshiftValidator) ValidateDatabaseSchema() error {
	if v.DatabaseSchema == "" {
		return errors.New("DatabaseSchema definition missing from Redshift configuration. More information: https://github.com/fullstorydev/hauser/blob/master/Redshift.md#database-schema-configuration")
	}
	return nil
}

type GCSConfig struct {
	Bucket  string
	GCSOnly bool
}

type BigQueryConfig struct {
	Project     string
	Dataset     string
	ExportTable string
	SyncTable   string
}

type duration struct {
	time.Duration
}

func (d *duration) UnmarshalText(text []byte) error {
	var err error
	d.Duration, err = time.ParseDuration(string(text))
	return err
}

func Load(filename string) (*Config, error) {
	var conf Config

	tomlData, err := ioutil.ReadFile(filename)
	if err != nil {
		return nil, err
	}

	if _, err := toml.Decode(string(tomlData), &conf); err != nil {
		return nil, err
	}

	conf.Redshift.Validator = &RedshiftValidator{
		RedshiftConfigFields: conf.Redshift.RedshiftConfigFields,
	}


	return &conf, nil
}
