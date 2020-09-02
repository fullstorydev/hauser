package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"cloud.google.com/go/storage"
	"github.com/fullstorydev/hauser/client"
	"github.com/fullstorydev/hauser/config"
	"github.com/fullstorydev/hauser/internal"
	"github.com/fullstorydev/hauser/warehouse"
)

var version = "dev build <no version set>"

func MakeStorage(ctx context.Context, conf *config.Config) warehouse.Storage {
	switch conf.Provider {
	case "local":
		return warehouse.NewLocalDisk(&conf.Local)
	case "aws":
		return warehouse.NewS3Storage(&conf.S3)
	case "gcp":
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
	case "local":
		log.Fatalf("cannot initialize database for local provider")
	case "aws":
		return warehouse.NewRedshift(&conf.Redshift)
	case "gcp":
		return warehouse.NewBigQuery(&conf.BigQuery)
	default:
		log.Fatalf("unknown provider type: %s", conf.Provider)
	}
	return nil
}

func main() {
	conffile := flag.String("c", "config.toml", "configuration file")
	printVersion := flag.Bool("version", false, "print version")
	flag.Parse()

	if *printVersion {
		fmt.Printf("%s %s\n", filepath.Base(os.Args[0]), version)
		os.Exit(0)
	}

	conf, err := config.Load(*conffile)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	store := MakeStorage(ctx, conf)
	database := MakeDatabase(ctx, conf)
	hauser := internal.NewHauser(conf, client.NewClient(conf), store, database)
	hauser.Run(ctx)
}
