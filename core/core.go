package core

import (
	"context"
	"log"

	"cloud.google.com/go/storage"
	"github.com/fullstorydev/hauser/client"
	"github.com/fullstorydev/hauser/config"
	"github.com/fullstorydev/hauser/internal"
	"github.com/fullstorydev/hauser/warehouse"
)

func NewHauser(config *config.Config, fsClient client.DataExportClient, storage warehouse.Storage, db warehouse.Database) *internal.HauserService {
	return internal.NewHauserService(config, fsClient, storage, db)
}

func MakeStorage(ctx context.Context, conf *config.Config) warehouse.Storage {
	switch conf.Provider {
	case config.LocalProvider:
		return warehouse.NewLocalDisk(&conf.Local)
	case config.AWSProvider:
		return warehouse.NewS3Storage(&conf.S3)
	case config.GCProvider:
		gcsClient, err := storage.NewClient(ctx)
		if err != nil {
			log.Fatalf("Failed to create GCS client")
		}
		return warehouse.NewGCSStorage(&conf.GCS, gcsClient)
	default:
		log.Fatalf("unknown provider type: %s", conf.Provider)
	}
	return nil
}

func MakeDatabase(_ context.Context, conf *config.Config) warehouse.Database {
	if conf.StorageOnly {
		return nil
	}
	switch conf.Provider {
	case config.LocalProvider:
		log.Fatalf("cannot initialize database for local provider")
	case config.AWSProvider:
		if conf.Snowflake.ExportTable != "" {
			return warehouse.NewSnowflake(&conf.Snowflake)
		}
		return warehouse.NewRedshift(&conf.Redshift)
	case config.GCProvider:
		if conf.Snowflake.ExportTable != "" {
			return warehouse.NewSnowflake(&conf.Snowflake)
		}
		return warehouse.NewBigQuery(&conf.BigQuery)
	default:
		log.Fatalf("unknown provider type: %s", conf.Provider)
	}
	return nil
}
