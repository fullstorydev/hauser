#!/bin/bash
set -e

echo "Generating config..."
echo $GOOGLE_KEY_JSON > /server-conf/google_key.json
export GOOGLE_APPLICATION_CREDENTIALS=/server-conf/google_key.json

envsubst < /server-conf/config.toml.tmpl > /server-conf/config.toml
# cat /server-conf/config.toml
# cat /server-conf/google_key.json

echo "Starting hauser..."
hauser -c /server-conf/config.toml