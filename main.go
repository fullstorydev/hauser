package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/fullstorydev/hauser/client"
	"github.com/fullstorydev/hauser/config"
	"github.com/fullstorydev/hauser/resources"
	"log"
	"os"
	"path/filepath"
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
	store := resources.MakeStorage(ctx, conf)
	database := resources.MakeDatabase(ctx, conf)
	client := client.NewClient(conf)
	hauser := resources.NewHauser(conf, client, store, database)
	hauser.Run(ctx)
}
