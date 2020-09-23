package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/fullstorydev/hauser/client"
	"github.com/fullstorydev/hauser/config"
	"github.com/fullstorydev/hauser/hauser"
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
	store := hauser.MakeStorage(ctx, conf)
	database := hauser.MakeDatabase(ctx, conf)
	cl := client.NewClient(conf)
	h := hauser.NewHauser(conf, cl, store, database)
	h.Run(ctx)
}
