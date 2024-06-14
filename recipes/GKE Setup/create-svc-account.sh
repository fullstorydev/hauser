#!/bin/bash

set -e

SERVICE_EMAIL="$SERVICE_NAME@$GOOGLE_PROJECT.iam.gserviceaccount.com"

function setup_bq_perms() {
  filter=".access[]
 | select(.userByEmail != null)
 | select(.userByEmail | contains(\"$SERVICE_EMAIL\"))
 .role
"
  bqconfig=$(bq show --format=prettyjson "$BIGQUERY_DATASET")
  role=$(echo "$bqconfig" | jq -r "$filter")
  if [[ "$role" = "WRITER" ]];then
    echo "BigQuery permissions already in place"
    exit 0;
  fi

  echo "Service account should have 'WRITER' role, but doesn't. Adding now..."

  # Add hauser to the access list and filter out any existing hauser role.
  hauser_access="{
    \"role\":\"WRITER\",
    \"userByEmail\":\"$SERVICE_EMAIL\"
  }"
  add_stmt=".access |= [ . + [$hauser_access] | .[] | select(
    .userByEmail != null and
    (.userByEmail | contains(\"$SERVICE_EMAIL\")) and
    (.role != \"WRITER\")
    | not
  )]"
  # ensure the scratch file doesn't exist
  [[ -f /tmp/hauser-bq-update.json ]] && rm /tmp/hauser-bq-update.json

  bq show --format=prettyjson "$BIGQUERY_DATASET" | jq "$add_stmt" > /tmp/hauser-bq-update.json && \
  bq update --source /tmp/hauser-bq-update.json "$BIGQUERY_DATASET" && \
  rm /tmp/hauser-bq-update.json
}

echo "Checking existence of service account $SERVICE_NAME in project $GOOGLE_PROJECT"
echo "================================================="
if ! gcloud iam service-accounts describe "$SERVICE_EMAIL"
then
  read -p "Service account not found. Do you want to create one? (Y/n)" create
  if [[ $create = '' || $create = 'y' ]]; then
    if ! gcloud iam service-accounts create "$SERVICE_NAME" --project="$GOOGLE_PROJECT";
    then
      echo "Failed to create service account"
      exit 1
    fi
  else
    echo "Aborting setup"
    exit 1
  fi
fi

echo "================================================="
echo "Service account exists. Ensuring permissions are set..."

gcloud iam service-accounts add-iam-policy-binding "$SERVICE_EMAIL" \
 --member="serviceAccount:$GOOGLE_PROJECT.svc.id.goog[$K8S_NAMESPACE/$SERVICE_NAME]" \
 --role=roles/iam.workloadIdentityUser

gcloud projects add-iam-policy-binding "$GOOGLE_PROJECT" \
 --member="serviceAccount:$SERVICE_EMAIL" \
 --role=roles/bigquery.jobUser

echo "Ensuring serviceAccount has objectAdmin role for bucket '$GCS_BUCKET'"
gsutil iam ch "serviceAccount:$SERVICE_EMAIL:objectAdmin" "gs://$GCS_BUCKET"

echo "Ensuring serviceAccount has 'WRITER' access to BigQuery Dataset '$BIGQUERY_DATASET'"
setup_bq_perms
