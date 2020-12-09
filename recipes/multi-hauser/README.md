# Multi-Hauser with Docker

If your FullStory account has multiple associated accounts, this recipe can be used as a template to
start a docker container that exports the data for each "org" and uploads it to BigQuery.
While this is intended to work with Google Cloud, it can easily be adapted to work with
AWS or any databases supported in the future.

## How it works
This recipe works by creating a docker container that uses [supervisord] to run multiple `hauser`
processes against multiple FullStory accounts (i.e. "orgs").
When the docker container is started, it uses the provided [key file](#key-file) to build a `config.toml` for each org from the
[hauser config template](hauser-config.toml.tmpl) and saves it to a working directory for that org
(e.g. `/hauser/<orgId>/config.toml`).
A ["program"](http://supervisord.org/configuration.html#program-x-section-settings) is also added to the supervisor config
file.
Supervisor is then started and runs in the foreground until the container is terminated.

## Hauser Configuration Template
The template in this directory requires that three variables are defined:
 * `Bucket`: Google Cloud Storage bucket ([gcs] section)
 * `Project`: Google Cloud project ID ([bigquery] section)
 * `Dataset`: BigQuery dataset that the tables will be created in ([bigquery] section)

The other `hauser` configuration variables can be customized as well, but the above are the minimum required.

#### Export Delay
In an attempt to offset the times at which each process is doing work,
the generated `hauser` configs will have a 10 min offset in their `ExportDelay` value.
So, if your [key file][key file] has 3 lines, the `ExportDelay` value for each org will be
24 hours, 24 hours and 10 minutes, and 24 hours and 20 minutes, respectively.

## Key File
The key file should be a CSV with the following format:

```
<org1>,<api_key_for_org1>
<org2>,<api_key_for_org2>
<org3>,<api_key_for_org3>
...
```

## Building and running the docker container
Assuming the current working directory is the directory of this README,
the docker container can be built with the following command:

```bash
docker build -t multi-hauser . --build-arg HAUSER_VERSION=1.0.0
```

This builds a docker image that does not contain any sensitive information (such as API keys and GCP credentials), and
thus is safely portable/shareable.

Running the container is a bit more involved and can change based on how the deployment is being managed.
For instance, with Google Cloud, the container should use a service account so that `hauser` can make
API calls to Google Cloud Storage and BigQuery. In Google Kubernetes Engine (GKE), the credentials can easily
be stored in secrets and mounted to the docker container as specified in the deployment.
If running on a VM, the credentials can just be copied directly to the the desired location.

The following `docker run` command is just an example for running the process manually and assumes that the
[key file](#key-file) and service account credentials are in the current directory.

```bash
docker run --rm -it \
  -v ${PWD}/keys.csv:/secrets/keys.csv \
  -v ${PWD}/account.json:/secrets/account.json \
  -e GOOGLE_APPLICATION_CREDENTIALS=/secrets/account.json \
  --name multi-hauser
  multi-hauser:latest /secrets/keys.csv
```

[supervisord]: supervisord.org
[key file]: #key-file
