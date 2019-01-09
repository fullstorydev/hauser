package config

import (
	"io/ioutil"
	"time"

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

	// local filesystem: Local
	Local LocalConfig
}

type S3Config struct {
	Bucket  string
	Region  string
	Timeout duration
	S3Only  bool
}

type RedshiftConfig struct {
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

type LocalConfig struct {
	SaveDir      string
	StartTime    time.Time
	UseStartTime bool
	SaveAsJson   bool
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

	return &conf, nil
}
