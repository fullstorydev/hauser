# hauser

Branching here


`hauser` is a service to download FullStory data export files and load them into a data warehouse. (Redshift is the only warehouse supported currently, although others are easy to add -- pull requests welcome.)

## Quick Start

* Build it (for EC2, for example): ``GOPATH=`pwd` GOOS=linux GOARCH=amd64 go build ``
* Copy `example-config.toml` and customize it for your environment, including your FullStory API key, warehouse host and credentials. AWS credentials (for S3) come from your local environment.
* Run it: `./hauser -c <your updated config file>`

## How it works
When first run, `hauser` will query FullStory's [data export API](http://help.fullstory.com/develop-rest) to find the earliest export file available. `hauser` will then download all available export files, performing some light transformation for [custom user vars](http://help.fullstory.com/develop-js/setuservars?from_search=17717406) before loading it into the warehouse.

For Redshift, the export file is copied locally to the temp directory before it is moved to S3. The S3 copy is then loaded into Redshift through the `copy` command.

`hauser` will work through all available export files serially. When no further export files are available, `hauser` will sleep until there is a new one available, which will be processed immediately.

`hauser` can safely be stopped and restarted. For Redshift, it uses the `SyncTable` to keep track of what export files have been processed, and will restart from the last known sync point.

## Working with Custom Vars
For convenience, any custom user vars in your data are stored in a json map in the `CustomVars` column. In Redshift, they can be easily accessed using the [`JSON_EXTRACT_PATH_TEXT`](http://docs.aws.amazon.com/redshift/latest/dg/JSON_EXTRACT_PATH_TEXT.html) function.

For example:
```
SELECT COUNT(*)
FROM myexport
WHERE JSON_EXTRACT_PATH_TEXT(CustomVars, 'acct_adminDisabled_bool') = 'false';
```
