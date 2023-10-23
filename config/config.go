package config

import (
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"time"

	"github.com/BurntSushi/toml"
)

const (
	// DefaultApiURL is the standard base URL for the fullstory.com API.
	DefaultApiURL         = "https://api.fullstory.com"
	DefaultSegmentId      = "everyone"
	DefaultExportDelay    = 24 * time.Hour
	DefaultExportDuration = 1 * time.Hour
	MinExportDuration     = 15 * time.Minute
	MaxExportDuration     = 24 * time.Hour
)

type Provider string

const (
	LocalProvider Provider = "local"
	AWSProvider   Provider = "aws"
	GCProvider    Provider = "gcp"
)

type Config struct {
	// Deprecated: Use Provider instead
	Warehouse            string
	Provider             Provider
	FsApiToken           string
	ExportDuration       Duration
	ExportDelay          Duration
	AdditionalHttpHeader []Header
	Backoff              Duration
	BackoffStepsMax      int
	// Deprecated
	CheckInterval Duration
	TmpDir        string
	// Deprecated
	ListExportLimit int
	// Deprecated: use ExportDuration
	GroupFilesByDay bool
	SaveAsJson      bool
	StorageOnly     bool
	StartTime       time.Time

	// The segment to export. Defaults to the "everyone" segment, which will export all data.
	SegmentId string

	IncludeMobileAppsFields bool

	ApiURL string

	FilePrefix string

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

type StorageConfig struct {
	FilePrefix string `toml:"-"`
}

type S3Config struct {
	StorageConfig
	Bucket  string
	Region  string
	Timeout Duration
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
	StorageConfig
	Bucket string
	// Deprecated: Use `StorageOnly` option at the main level
	GCSOnly bool
}

type BigQueryConfig struct {
	Project             string
	Dataset             string
	ExportTable         string
	SyncTable           string
	PartitionExpiration Duration
}

type Duration struct {
	time.Duration
}

type LocalConfig struct {
	StorageConfig
	SaveDir string
	// Deprecated: Use `StartTime` in the base config instead
	StartTime time.Time
	// This forces the exports to start at the provided StartTime instead of using the
	// any sync file that might exist.
	UseStartTime bool
}

func (d *Duration) UnmarshalText(text []byte) error {
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

	if envToken := os.Getenv("FULLSTORY_API_TOKEN"); envToken != "" {
		conf.FsApiToken = envToken
	}

	if err := Validate(&conf, time.Now); err != nil {
		return nil, err
	}
	return &conf, nil
}

func Validate(conf *Config, getNow func() time.Time) error {
	// Set any defaults.
	if conf.ApiURL == "" {
		conf.ApiURL = DefaultApiURL
	}

	if conf.SegmentId == "" {
		conf.SegmentId = DefaultSegmentId
	}

	if conf.ExportDuration.Duration == 0 {
		if conf.GroupFilesByDay {
			log.Println(`WARNING: The "GroupFilesByDay" option is deprecated. Please use "ExportDuration" instead.`)
			conf.ExportDuration.Duration = 24 * time.Hour
		} else {
			log.Println(`INFO: "ExportDuration" not set in config. Defaulting to 1 hour`)
			conf.ExportDuration.Duration = DefaultExportDuration
		}
	} else if conf.ExportDuration.Duration < MinExportDuration || conf.ExportDuration.Duration > MaxExportDuration {
		return fmt.Errorf("ExportDuration '%s' out of range. The range of valid values is from %s to %s", conf.ExportDuration.Duration, MinExportDuration, MaxExportDuration)
	} else if (24*time.Hour)%conf.ExportDuration.Duration != 0 {
		// The Duration needs to fit evenly within a day so that database partitioning by day
		// works correctly
		return errors.New("ExportDuration must be an even fraction of 24 hours")
	}

	if conf.ExportDelay.Duration == 0 {
		conf.ExportDelay.Duration = DefaultExportDelay
	} else if conf.ExportDelay.Duration < time.Hour {
		return errors.New(`"ExportDelay" configuration value is too small. Minimum value is 1 hour`)
	}

	// Ensure a sane start time and make sure it's in UTC
	if conf.StartTime.IsZero() {
		log.Println(`INFO: "StartTime" not set in config. Defaulting to 30 days in the past`)
		conf.StartTime = getNow().UTC().Add(-1 * 24 * 30 * time.Hour)
	}
	conf.StartTime = conf.StartTime.UTC()

	if conf.BigQuery.PartitionExpiration.Duration < time.Duration(0) {
		return errors.New("BigQuery expiration value must be positive")
	}

	if conf.Provider == "" {
		switch conf.Warehouse {
		case "local":
			conf.Provider = LocalProvider
		case "redshift":
			conf.Provider = AWSProvider
		case "bigquery":
			conf.Provider = GCProvider
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

	switch conf.Provider {
	case LocalProvider:
		// The local provider only supports storage
		log.Println(`WARNING: The "local" provider only supports "StorageOnly = true".
          This value will be ignored in your configuration file.`)
		conf.StorageOnly = true
		conf.Local.FilePrefix = conf.FilePrefix
	case AWSProvider:
		conf.StorageOnly = conf.StorageOnly || conf.S3.S3Only

		if !conf.StorageOnly {
			// Redshift needs to know which region the storage is in. Make sure they match
			conf.Redshift.S3Region = conf.S3.Region
		}
		conf.S3.S3Only = false
		conf.S3.FilePrefix = conf.FilePrefix
	case GCProvider:
		conf.StorageOnly = conf.StorageOnly || conf.GCS.GCSOnly
		conf.GCS.GCSOnly = false
		conf.GCS.FilePrefix = conf.FilePrefix
	}

	if conf.SaveAsJson && !(conf.Provider == "local" || conf.StorageOnly) {
		return fmt.Errorf("hauser doesn't currently support loading JSON into a database. Ensure SaveAsJson = false in .toml file")
	}
	return nil
}
