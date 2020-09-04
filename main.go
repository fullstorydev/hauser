package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

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

	var wh warehouse.Warehouse
	switch conf.Warehouse {
	case "local":
		wh = warehouse.NewLocalDisk(conf)
	case "redshift":
		wh = warehouse.NewRedshift(conf)
		if conf.SaveAsJson {
			if !conf.S3.S3Only {
				log.Fatalf("Hauser doesn't currently support loading JSON into Redshift.  Ensure SaveAsJson = false in .toml file.")
			}
		}
	case "bigquery":
		wh = warehouse.NewBigQuery(conf)
		if conf.SaveAsJson {
			if !conf.GCS.GCSOnly {
				log.Fatalf("Hauser doesn't currently support loading JSON into BigQuery.  Ensure SaveAsJson = false in .toml file.")
			}
		}
	default:
		if len(conf.Warehouse) == 0 {
			log.Fatal("Warehouse type must be specified in configuration")
		} else {
			log.Fatalf("Warehouse type '%s' unrecognized", conf.Warehouse)
		}
	}

	hauser := internal.NewHauser(conf, client.NewClient(conf), wh)
	hauser.Run()
}
