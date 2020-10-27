# hauser
`hauser` is a service to download FullStory Data Export files and load them into storage.
Currently, Data Export files can be saved to local disk, S3, Redshift, GCS, and BigQuery.
(Others are easy to add -- pull requests welcome.)

`hauser` is designed to run continuously so that it can update your chosen data store as new data becomes available.
 VMs are a good option for running `hauser` continuously.

SQL recipes for Data Export analysis are in the [Data Export Cookbook](https://github.com/fullstorydev/hauser/wiki).
<p>
  <br>
</p>

<p align="center">
  <img width="414" src="fullstory_logo_color.png" alt="fullstory logo"/>
</p>

## Quick Start
1. Download the latest [release binary](https://github.com/fullstorydev/hauser/releases)
2. Download the included `example-config.toml` file and customize it for your environment,
   including your FullStory API key, warehouse host, and credentials. AWS credentials (for S3) come from your local environment.
3. Assuming the binary and updated config are in the current directory, run:
```bash
./hauser -c myconfig.toml
```

### Important Configuration Fields

#### `ExportDuration`
Determines the time range for each export which ultimately determines the size of each exported file.
The size of this file will be based on the amount of traffic that your FullStory account records within the
specified duration.
The default is 1 hour, but if a different file size is desirable, this can be modified to meet your specific needs. The
max duration is 24 hours.

#### `ExportDelay`
Determines how long to wait before creating an export.
This delay is necessary because there is some latency between when an event is recorded and when it is available and complete.
24 hours is the default, but is also fairly conservative. In many cases this can be reduced safely to 3 hours, but note that
events from "[swan songs]" may not be available.

#### `StartTime`
Determines the datetime that should be used a starting point for creating the exports.
This value is only used when starting with a fresh database/storage instance (i.e. `hauser` hasn't been used with the specified warehouse).
If you would like to export all the data that is currently within retention, set this to the date of the oldest possible
session start. For example, if your FullStory account has 3 months of retention, and today is October 20th, 2020, set `StartTime`
to `2020-7-20T00:00:00Z` to include the oldest data for your account.

## How It Works
`hauser` will use FullStory's [segment export API] to create exports
of the `everyone` segment. When the export has completed (see [Operations API](https://developer.fullstory.com/get-operation)),
`hauser` will download the file, perform some light transformation for [custom user vars](http://help.fullstory.com/develop-js/setuservars?from_search=17717406)
, and load the data into the warehouse.

`hauser` will continue to this process of "create export -> download -> upload" until it reaches the most "live" data (i.e. "Now - `ExportDelay` - `ExportDuration`").
At this point, `hauser` will perform the process approximately every `ExportDuration`.

`hauser` can safely be stopped and restarted.
When using a database, it uses the `SyncTable` to keep track of what export files have been processed, and will restart from the last known sync point.
For a `StorageOnly` process, it will create a file called `.sync.hauser` that will be used as a checkpoint.

### Amazon Web Services Notes
_Currently, only S3 and Redshift are supported for this provider._

To use AWS, set the `Provider` config option to `aws`.

Each export file is saved locally to the temp directory before it is moved to S3.
If not `StorageOnly`, the S3 copy is then loaded into Redshift through the `copy` command, and the S3 file is removed.

Details about Redshift configuration can be found in the [Redshift Guide](https://github.com/fullstorydev/hauser/blob/master/Redshift.md).

### Google Cloud Notes
_Currently, only GCS and BigQuery are supported for this provider._

To use Google Cloud, set the `Provider` config option to `gcp`.

Each export file is saved locally to the temp directory before it is moved to GCS.
If not `StorageOnly`, the GCS file is then loaded into BigQuery through the gRPC client API equivalent of the `bq load` command,
and the GCS file is removed.

The BigQuery `ExportTable` is expected to be a date partitioned table.
The default values `ExportTable = "fs_export"` and `SyncTable = "fs_sync"` will work, but feel free to customize the `fs_sync` and `fs_export` names.
If the `SyncTable` and `ExportTable` do not already exist in BigQuery, they will be created.

### Local Storage Notes

To only store downloaded export files locally, set the `Provider` option to `local`.
This will save exports to a local folder specified by `SaveDir`.
If `UseStartTime` is set to `true`, only exports since `StartTime` will be downloaded (as opposed to all available exports).
Exports can be saved in JSON format (by setting `SaveAsJson` to `true`) or in CSV format.

## Table Schema Changes

On startup, `hauser` will ensure that the export table listed in the config contains columns for all export fields.
If `hauser` detects columns for fields don't exist, it will append columns for those fields to the export table.
It uses this schema information, which it acquires once on startup, to intelligently build CSV files and deal with schema alterations to the export table.
If schema changes are made, `hauser` will have to be restarted so it is aware of the updated export table schema.

If the export table contains columns that aren't part of the export, `hauser` will insert null values for those columns when it inserts new records.
Note: In order for `hauser` to successfully insert records, any added columns must be nullable.

If FullStory adds fields to the export, a new version of hauser will need to be downloaded to pick up the new fields.
If a backfill of the fields is desired, you can create a one-off export of just the new fields by using the [segment export API].

## Working with Custom Vars
For convenience, any custom user vars in your data are stored in a json map in the `CustomVars` column. In Redshift, they can be easily accessed using the [`JSON_EXTRACT_PATH_TEXT`](http://docs.aws.amazon.com/redshift/latest/dg/JSON_EXTRACT_PATH_TEXT.html) function.

For example:
```
SELECT COUNT(*)
FROM myexport
WHERE JSON_EXTRACT_PATH_TEXT(CustomVars, 'acct_adminDisabled_bool') = 'false';
```

## Building from source
* Make sure you have [installed](https://golang.org/doc/install) Go 1.11 or higher.
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

## Developing
Easily format your commits by adding git pre-commit hook:
```bash
ln -s ../../pre-commit.sh .git/hooks/pre-commit
```

[segment export API]: http://developer.fullstory.com/create-segment-export
[swan songs]: https://help.fullstory.com/hc/en-us/articles/360048109714-Swan-songs-How-FullStory-records-sessions-that-end-unexpectedly
