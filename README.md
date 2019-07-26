# hauser
`hauser` is a service to download FullStory Data Export files and load them into a data warehouse. Alternatively, Data Export files can be saved to local disk. Redshift and BigQuery are the only warehouses supported currently. (Others are easy to add -- pull requests welcome.)

SQL recipes for Data Export analysis are in the [Data Export Cookbook](https://github.com/fullstorydev/hauser/wiki).

<p align="center">
  <img width="414" src="fullstory text.png" alt="fullstory"/>
</p>

## Quick Start
* Make sure you have [installed](https://golang.org/doc/install) Go 1.9 or higher.
* **OPTIONAL**: Set a custom [GOPATH](https://github.com/golang/go/wiki/SettingGOPATH).
* Build it... 
    * To compile for use on your local machine: ``go get github.com/fullstorydev/hauser``
    * To cross-compile for deployment on a VM: ``GOOS=<linux> GOARCH=<amd64> go get github.com/fullstorydev/hauser``
        - Type `go version` in the VM's command line to find its `GOOS` and `GOARCH` values.
        - Example (Amazon EC2 Linux): `go1.11.5 linux/amd64` is `GOOS=linux GOARCH=amd64`
        - The list of valid `GOOS` and `GOARCH` values can be found [here](https://golang.org/doc/install/source#environment).
* Copy the included `example-config.toml` file and customize it for your environment, including your FullStory API key, warehouse host, and credentials. AWS credentials (for S3) come from your local environment.
* Run it...
    * **NOTE**: `go get` downloads and installs the hauser package in your `GOPATH`, not the local directory in which you call the command.
    * If you did _NOT_ set a custom `GOPATH`...
        - Linux & Mac: `$HOME/go/bin/hauser -c <your updated config file>`
        - Windows: `%USERPROFILE%\go\bin\hauser -c <your updated config file>`
    * If you _DID_ set a custom `GOPATH`...
        - Linux & Mac: `$GOPATH/bin/hauser -c <your updated config file>`
        - Windows: `$GOPATH\bin\hauser -c <your updated config file>`
    
## How It Works
When first run, `hauser` will query FullStory's [data export API](http://help.fullstory.com/develop-rest) to find the earliest export file available. `hauser` will then download all available export files, performing some light transformation for [custom user vars](http://help.fullstory.com/develop-js/setuservars?from_search=17717406) before loading it into the warehouse.

`hauser` will work through all available export files serially. When no further export files are available, `hauser` will sleep until there is a new one available, which will be processed immediately.

Export files may be processed one at a time, or they may be grouped into batches by day using the boolean config option `GroupFilesByDay`.  When grouping is enabled, export files are still processed serially, but all files having the same date (in UTC) will be combined into a single file before upload to the target warehouse.  Grouping files is helpful for loading large amounts of historical data, when the total number of load operations might reach some quota.  BigQuery, for example, limits the number of loads per day on a single table to 1000.

`hauser` can safely be stopped and restarted. For Redshift and BigQuery, it uses the `SyncTable` to keep track of what export files have been processed, and will restart from the last known sync point.

### Redshift Notes
To use the Redshift warehouse, set the `Warehouse` config option to `redshift`.

By default, each export file is copied locally to the temp directory before it is moved to S3. The S3 copy is then loaded into Redshift through the `copy` command.  Finally, the S3 copy of the file is removed.

Loading data into Redshift may be skipped by setting `S3.S3Only` in the config file to `true`. In this mode, files are copied to S3, where they remain without being loaded into Redshift.

Details about Redshift configuration can be found in the [Redshift Guide](https://github.com/fullstorydev/hauser/blob/master/Redshift.md).

### BigQuery Notes
To use the BigQuery warehouse, set the `Warehouse` config option to `bigquery`.

By default, each export file is copied locally to the temp directory before it is moved to GCS. The GCS copy is then loaded into BigQuery through the gRPC client API equivalent of the `bq load` command.

The BigQuery `ExportTable` is expected to be a date partitioned table. The default values `ExportTable = "fs_export"` and `SyncTable = "fs_sync"` will work, but feel free to customize the `fs_sync` and `fs_export` names.  If the `SyncTable` and `ExportTable` do not already exist in BigQuery, they will be created on the fly, without an expiration time for the partitions. Once a file is loaded into BigQuery, the GCS copy of the file is removed.

Loading data into BigQuery may be skipped by setting `GCS.GCSOnly` in the config file to `true`. In this mode, files are copied to GCS, where they remain without being loaded into BigQuery.

If `hauser` detects that a load failure occurred, to ensure data consistency it will roll back all sync points for the most recent date partition and reload all files for the entire partition.

### Local Storage Notes

To only store downloaded export files locally, set the `Warehouse` option to `local`. This will save exports to a local folder specified by `SaveDir`. If `UseStartTime` is set to `true`, only exports since `StartTime` will be downloaded (as opposed to all available exports). Exports can be saved in JSON format (by setting `SaveAsJson` to `true`) or in CSV format. `GroupFilesByDay` option is also supported (for CSV format only).


## Table Schema Changes

As FullStory adds more features we expose additional fields in our data export. `hauser` automatically deals with the addition of new fields by appending nullable columns to the warehouse export table.

On startup, `hauser` will ensure that the export table listed in the config contains columns for all export fields. If `hauser` detects columns for fields don't exist, it will append columns for those fields to the export table. It uses this schema information, which it acquires once on startup, to intelligently build CSV files and deal with schema alterations to the export table. If schema changes are made, `hauser` will have to be restarted so it is aware of the updated export table schema.

If the export table contains columns that aren't part of the export bundle, `hauser` will insert null values for those columns when it inserts new records. Note: In order for `hauser` to successfully insert records, any added columns must be nullable.

## Working with Custom Vars
For convenience, any custom user vars in your data are stored in a json map in the `CustomVars` column. In Redshift, they can be easily accessed using the [`JSON_EXTRACT_PATH_TEXT`](http://docs.aws.amazon.com/redshift/latest/dg/JSON_EXTRACT_PATH_TEXT.html) function.

For example:
```
SELECT COUNT(*)
FROM myexport
WHERE JSON_EXTRACT_PATH_TEXT(CustomVars, 'acct_adminDisabled_bool') = 'false';
```

## Developing
Easily format your commits by adding git pre-commit hook:
```bash
ln -s ../../pre-commit.sh .git/hooks/pre-commit
```
