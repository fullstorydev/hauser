#!/bin/bash

set -e

GOOGLE_PROJECT=""
ORG_ID=""
BIGQUERY_DATASET="fullstory-export-data"
SERVICE_NAME="hauser-svc"
GCS_BUCKET="$GOOGLE_PROJECT-hauser"
HAUSER_VERSION="latest"
K8S_NAMESPACE="default"

function usage() {
    echo "Usage: deploy.sh -p GOOGLE_PROJECT -o ORG_ID -k FS_API_KEY [-n SERVICE_NAME ]"
}

while getopts "hn:p:o:k:" opt; do
  case $opt in
    n)
      SERVICE_NAME=$OPTARG
      ;;
    p)
      GOOGLE_PROJECT=$OPTARG
      ;;
    o)
      ORG_ID=$OPTARG
      ;;
    k)
      FS_API_KEY=$OPTARG
      ;;
    h)
      usage
      exit 0
      ;;
    \?)
      echo "Invalid option: -$OPTARG" >&2
      exit 1
      ;;
    :)
      echo "Option -$OPTARG requires an argument." >&2
      exit 1
      ;;
  esac
done

if [[ -z "$ORG_ID" ]]; then
  echo "Missing -o (org_id) option"
  usage
  exit 1
fi

if [[ -z "$GOOGLE_PROJECT" ]]; then
  echo "Missing -p (project_id) option"
  usage
  exit 1
fi

if [[ -z "$BIGQUERY_DATASET" ]]; then
  echo "BIGQUERY_DATASET variable not set"
  usage
  exit 1
fi

if [[ -z "$FS_API_KEY" ]]; then
  echo "Missing -k (api key) option"
  exit 1
fi

# export all the variables for use in `create-svc-account.sh` and `envsubst`
export SERVICE_NAME
export GOOGLE_PROJECT
export ORG_ID
export BIGQUERY_DATASET
export GCS_BUCKET
export HAUSER_VERSION
export K8S_NAMESPACE

/bin/bash ./create-svc-account.sh

echo "================================================="
echo "Running kubernetes diff"

K8S_CONFIG=$(envsubst < ./hauser.yaml)
DIFF=$(echo "$K8S_CONFIG" | kubectl diff -f - || true)

if [[ $DIFF = "" ]]; then
  echo "Nothing to do, deployment up to date"
  exit 0;
fi

echo "================================================="
echo "The following changes will be applied"
echo ""
echo "$DIFF"
echo "================================================="
read -r -p "ok to continue? (y/N) " continue

if [[ $continue = "y" ]]; then
  echo "Applying kubernetes config"
  echo "$K8S_CONFIG" | kubectl apply -f -
else
  echo "Aborting deploy"
  exit 1;
fi
