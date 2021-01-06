#!/bin/sh
set -e

TMPL_DIR=/tmpl
export HAUSER_WORKDIR=/hauser
export CONF_DIR=/conf

if [ ! -f "$1" ]; then
    echo "Key file $1 does not exist."
    exit 1
fi

# Log level for supervisord
export LOGLEVEL="${SUPERVISORD_LOGLEVEL:=info}"

# Create a new supervisord.conf
echo "Creating supervisord.conf"
envsubst < "$TMPL_DIR/supervisord.conf.tmpl" > "$CONF_DIR/supervisord.conf"

# Build corresponding *.toml config files and add supervisord program for each org
index=0
while IFS=, read -r FULLSTORY_ORGID FULLSTORY_APIKEY
do
  export FULLSTORY_ORGID=$FULLSTORY_ORGID
  export FULLSTORY_APIKEY=$FULLSTORY_APIKEY
  echo "Creating Hauser configuration for $FULLSTORY_ORGID"

  # Offset delay by 10 min to prevent downloads occuring at the same time
  export HAUSER_EXPORT_DELAY="$((1440 + (10 * index)))m"

  # Create the temp dir to be used for the donwloaded files
  mkdir -p "$HAUSER_WORKDIR/$FULLSTORY_ORGID/tmp"
  envsubst < "$TMPL_DIR/hauser-config.toml.tmpl" > "$HAUSER_WORKDIR/$FULLSTORY_ORGID/config.toml"

  echo "Adding Hauser program to supervisord configuration for $FULLSTORY_ORGID"
  envsubst < "$TMPL_DIR/supervisor-program.conf.tmpl" >> "$CONF_DIR/supervisord.conf"
  index=$((index+1))
done < "$1"

/usr/bin/supervisord -c $CONF_DIR/supervisord.conf
