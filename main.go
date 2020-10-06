package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"

	"github.com/fullstorydev/hauser/client"
	"github.com/fullstorydev/hauser/config"
	"github.com/fullstorydev/hauser/core"
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
	store := core.MakeStorage(ctx, conf)
	database := core.MakeDatabase(ctx, conf)
	cl := client.NewClient(conf)
	h := core.NewHauser(conf, cl, store, database)
	h.Run(ctx)
}
