package config

import (
	"errors"
	"io/ioutil"
	"time"

	"github.com/BurntSushi/toml"
)

// DefaultExportURL is the standard base URL for the fullstory.com API.
const DefaultExportURL = "https://export.fullstory.com/api/v1"

type Config struct {
	Warehouse            string
	FsApiToken           string
	AdditionalHttpHeader []Header
	Backoff              duration
	BackoffStepsMax      int
	CheckInterval        duration
	TmpDir               string
	ListExportLimit      int
	GroupFilesByDay      bool
	SaveAsJson           bool

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

type Header struct {
	Key   string
	Value string
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
	Project             string
	Dataset             string
	ExportTable         string
	SyncTable           string
	PartitionExpiration duration
}

type duration struct {
	time.Duration
}

type LocalConfig struct {
	SaveDir      string
	StartTime    time.Time
	UseStartTime bool
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

	// Set any defaults.
	if conf.ExportURL == "" {
		conf.ExportURL = DefaultExportURL
	}

	if conf.BigQuery.PartitionExpiration.Duration < time.Duration(0) {
		return nil, errors.New("BigQuery expiration value must be positive")
	}

	return &conf, nil
}
