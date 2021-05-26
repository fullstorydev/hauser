# Google Kubernetes Engine (GKE) Setup

There are many ways to set up hauser to run in a cloud environment; this guide does not
aim to be comprehensive, but rather to provide a recommended configuration for a simple workflow.

In order to be able to set up the infrastructure for your project, it's best if you have at an [Editor
role](https://cloud.google.com/iam/docs/understanding-roles#basic). Else, you will need permission to
create/edit the following resources:
* Google Cloud Storage (GCS) buckets
* BigQuery Datasets
* GKE Kubernetes objects
* Service Accounts
* Service Account IAM Policies

Ensure that you have the [Google Cloud SDK](https://cloud.google.com/sdk/docs/install) and `kubectl` installed
and that you've [enabled cluster access](https://cloud.google.com/kubernetes-engine/docs/how-to/cluster-access-for-kubectl).

To run the `./deploy.sh` script, you must also have the following CLI tools installed and in your $PATH
* `gsutil` - Storage CLI (installable with `gcloud components install gsutil`)
* `bq` - BigQuery CLI (installable with `gcloud components install bq`)
* [`jq`](https://stedolan.github.io/jq/) - JSON processor

## Prerequisites
As described in the [cluster access documentation](https://cloud.google.com/kubernetes-engine/docs/how-to/cluster-access-for-kubectl),
first, ensure that your gcloud SDK is initialized:
```shell
gcloud init
```
This will authenticate the SDK, allow you to select a project, and configure a default zone/region.

Then (assuming you are using an existing cluster) you must generate a kubernetes configuration file:
```shell
gcloud container clusters get-credentials "<cluster-id>"
```
Before running the `deploy.sh` script, you must
[create a BigQuery dataset](https://cloud.google.com/bigquery/docs/datasets#create-dataset) (or use an existing dataset)
and a [Google Storage bucket](https://cloud.google.com/storage/docs/creating-buckets).

As a default, the script expects the name of the dataset to be `fullstory-export-data` and the
name of the bucket to be `$GOOGLE_PROJECT-hauser`, but feel free to change these.

If you did not use the default dataset and bucket names, or want to customize the name of the service account,
feel free to edit the following variables at the top of the `deploy.sh` script.
```shell
BIGQUERY_DATASET="fullstory-export-data"
SERVICE_NAME="hauser-svc"
GCS_BUCKET="$GOOGLE_PROJECT-hauser"
HAUSER_VERSION="latest"
```
To find your FullStory Org Id, see [help docs](https://help.fullstory.com/hc/en-us/articles/360047075853-How-do-I-find-my-FullStory-Org-Id-).
You will also need to create or get a [FullStory API Key](https://help.fullstory.com/hc/en-us/articles/360020624834-Where-can-I-find-my-API-key-).

The `HAUSER_VERSION` variable corresponds to the docker image tag. See [docker hub](https://hub.docker.com/r/fullstorydev/hauser) for available tags.

## The deploy script
The `deploy.sh` script in this directory uses the gcloud SDK and associated CLI's to
create a service account, add the permissions, and create a Kubernetes deployment that
runs hauser.

Assuming you have completed the prerequisites, you can run the deploy script with:
```shell
./deploy.sh -p "<my_project_id>" -o "<my_org_id>" -k "<fs_api_key"
```

Note: The script is idempotent, so if it fails, you can simply run it again after resolving any issues.

If a service account does not exist, the script will confirm before creating it. Assuming the service account creation
succeeded, the kubernetes included `hauser.yaml` will be rendered with the provided variables and diffed against your
current GKE cluster. The script will ask for confirmation before applying the diff.

If successful, you can check that the hauser pod is running with:
```shell
kubectl get pods -lapp=hauser -c hauser
```
which should return with something like
```
NAME                     READY   STATUS    RESTARTS   AGE
hauser-<pod_id_suffix>   2/2     Running   0          1m
```

To verify that everything is working as expected, you can also check the logs with
```shell
kubect logs -lapp=hauser -c hauser
```

## Service accounts
This setup is based on using a service account for the hauser deployment.
In short, this is recommended to adhere to the principal of east privilege.
To read more about service accounts in Google, see [Google's documentation](https://cloud.google.com/iam/docs/service-accounts)

The `create-svc-account.sh` script will create/validate that the service account
exists and has all of the necessary permissions and IAM policies associated with it.
This includes:
* [Workload Identity](https://cloud.google.com/kubernetes-engine/docs/how-to/workload-identity) to associate the gcloud
  service account with the Kubernetes service account.
* BigQuery Job User Role to allow hauser to load the data into bigquery (see [BigQuery roles](https://cloud.google.com/bigquery/docs/access-control))
* BigQuery Dataset "Writer" to allow hauser to create and insert data into tables ([docs](https://cloud.google.com/bigquery/docs/access-control-basic-roles#dataset-basic-roles))
* Object Admin for the Google Storage bucket to allow creating and deleting objects ([docs](https://cloud.google.com/storage/docs/access-control/iam-roles))

While using a service account is not necessary for a minimal deployment, it is highly recommended over using the
[default service account](https://cloud.google.com/iam/docs/service-accounts#default).

## Customizing hauser
If you would like to customize the configuration for hauser, feel free to edit the configmap that is described in
`hauser.yaml`. The config provided is complete and should operate well by default, but may not be exactly right for your needs.
