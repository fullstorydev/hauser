# AWS Redshift configuration details
Please review the `[redshift]` section in [example-config.toml](https://github.com/fullstorydev/hauser/blob/master/example-config.toml) to understand which items should be configured to integrate with your Redshift cluster. Values must be provided for all items.

Core configuration items are:

1. The Redshift credentials required to login to your Redshift host
2. The names of your Export and Sync tables. Tables will be created on your behalf if necessary (assuming that the provided credentials have been granted CREATE permissions on a schema)
3. The AWS IAM Role arn that will be used to import files from S3 into Redshift
4. The database schema used when querying (and creating) the Export and Sync tables

## Details about database schema configuration

The `DatabaseSchema` parameter can accept two different types of values:

1. A schema that exists in your Redshift cluster (including the "public" schema)
2. "search_path"

If "search_path" is provided, hauser will use your database's [search_path](https://docs.aws.amazon.com/redshift/latest/dg/r_search_path.html) configuration to determine which schema to use when accessing and creating tables.