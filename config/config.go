package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"time"

	"github.com/BurntSushi/toml"
)

// DefaultExportURL is the standard base URL for the fullstory.com API.
const DefaultExportURL = "https://export.fullstory.com/api/v1"

type Config struct {
	// Deprecated: Use Provider instead
	Warehouse            string
	Provider             string
	FsApiToken           string
	AdditionalHttpHeader []Header
	Backoff              duration
	BackoffStepsMax      int
	CheckInterval        duration
	TmpDir               string
	ListExportLimit      int
	GroupFilesByDay      bool
	SaveAsJson           bool
	StorageOnly          bool

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
	// Deprecated: Use `StorageOnly` option instead
	S3Only bool
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
	S3Region       string `toml:"-"`
}

type GCSConfig struct {
	Bucket string
	// Deprecated: Use `StorageOnly` option at the main level
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

	if err := Validate(&conf); err != nil {
		return nil, err
	}
	return &conf, nil
}

func Validate(conf *Config) error {
	// Set any defaults.
	if conf.ExportURL == "" {
		conf.ExportURL = DefaultExportURL
	}

	if conf.BigQuery.PartitionExpiration.Duration < time.Duration(0) {
		return errors.New("BigQuery expiration value must be positive")
	}

	if conf.Provider == "" {
		switch conf.Warehouse {
		case "local":
			conf.Provider = "local"
		case "redshift":
			conf.Provider = "aws"
		case "bigquery":
			conf.Provider = "gcp"
		default:
			if len(conf.Warehouse) == 0 {
				return fmt.Errorf("warehouse type must be specified in configuration")
			} else {
				return fmt.Errorf("warehouse type '%s' unrecognized", conf.Warehouse)
			}
		}
		log.Println(`WARNING: The "Warehouse" option is deprecated. Please use "Provider" instead.`)
		conf.Warehouse = ""
	}

	if conf.SaveAsJson && conf.Provider != "local" {
		return fmt.Errorf("hauser doesn't currently support loading JSON into a database. Ensure SaveAsJson = false in .toml file")
	}

	switch conf.Provider {
	case "local":
		// The local provider only supports storage
		log.Println(`WARNING: The "local" provider only supports "StorageOnly = true" and "SaveAsJson = true".
          These values will be ignored in your configuration file.`)
		conf.StorageOnly = true
		conf.SaveAsJson = true
	case "aws":
		conf.StorageOnly = conf.StorageOnly || conf.S3.S3Only
		conf.S3.S3Only = false
	case "gcp":
		conf.StorageOnly = conf.StorageOnly || conf.GCS.GCSOnly
		conf.GCS.GCSOnly = false
	}

	// Redshift needs to know which region the storage is in. Make sure they match
	if conf.Provider == "aws" && !conf.StorageOnly && conf.S3.Region != "" {
		conf.Redshift.S3Region = conf.S3.Region
	}
	return nil
}
