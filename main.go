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

	var store warehouse.Storage
	var database warehouse.Database
	switch conf.Provider {
	case "local":
		store = warehouse.NewLocalDisk(&conf.Local)
	case "aws":
		store = warehouse.NewS3Storage(&conf.S3)
		if !conf.StorageOnly {
			database = warehouse.NewRedshift(&conf.Redshift)
		}
	case "gcp":
		gcsClient, err := storage.NewClient(ctx)
		if err != nil {
			log.Fatalf("Failed to create GCS client")
		}

		store = warehouse.NewGCSStorage(&conf.GCS, gcsClient)
		if !conf.StorageOnly {
			database = warehouse.NewBigQuery(&conf.BigQuery)
		}
	default:
		log.Fatalf("unknown provider type: %s", conf.Provider)
	}

	hauser := internal.NewHauser(conf, client.NewClient(conf), store, database)
	hauser.Run(ctx)
}
