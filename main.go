package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"

	"github.com/fullstorydev/hauser/config"
	"github.com/fullstorydev/hauser/warehouse"
)

var version = "dev build <no version set>"

func main() {
	// conffile := flag.String("c", "config.toml", "configuration file")
	conffile := "bwamp.toml"
	printVersion := flag.Bool("version", false, "print version")
	flag.Parse()

	if *printVersion {
		fmt.Printf("%s %s\n", filepath.Base(os.Args[0]), version)
		os.Exit(0)
	}

	conf, err := config.Load(conffile)
	if err != nil {
		log.Fatal(err)
	}

	// ctx := context.Background()
	// store := core.MakeStorage(ctx, conf)
	// database := core.MakeDatabase(ctx, conf)
	// cl := client.NewClient(conf)
	// h := core.NewHauser(conf, cl, store, database)
	// h.Run(ctx)

	bq := warehouse.NewBigQuery(&conf.BigQuery)
	fmt.Printf(`["%s"]`, strings.Join(bq.GetExportTableColumns(), `", "`))

}
