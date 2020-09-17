package main

import (
	"context"
	"flag"
	"github.com/fullstorydev/hauser/config"
	"github.com/fullstorydev/hauser/resources"
	"log"
)

func main() {
	conffile := flag.String("c", "config.toml", "configuration file")
	printVersion := flag.Bool("version", false, "print version")
	flag.Parse()

	if *printVersion {
		config.PrintVersion()
	}

	conf, err := config.Load(*conffile)
	if err != nil {
		log.Fatal(err)
	}

	ctx := context.Background()
	store := resources.MakeStorage(ctx, conf)
	database := resources.MakeDatabase(ctx, conf)
	client := resources.MakeClient(ctx, conf)
	hauser := resources.NewHauser(conf, client, store, database)
	hauser.Run(ctx)
}
