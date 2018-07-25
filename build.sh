#!/bin/bash

app=hauser
container=c8-cluster
environ=prod
force=false
port=8080
project=c8-platform
region=us-central1
version=1.0.0
zone=us-central1-a

while [ $# -gt "0" ];
do
  case $1 in
    -a=*|--app=*) app="${1#*=}";;
    -c=*|--container=*) container="${1#*=}";;
    -e=*|--env=*) environ="${1#*=}";;
    -f=*|--force=*) force="${1#*=}";;
    -o=*|--port=*) port="${1#*=}";;
    -p=*|--project=*) project="${1#*=}";;
    -r=*|--region=*) region="${1#*=}";;
    -v=*|--version=*) version="${1#*=}";;
    -z=*|--zone=*) zone="${1#*=}";;
  esac
  shift
done;

echo "Building "$app":"$version" in "$project"-"$environ"("$region"/"$zone"):"$container

# ensure we're looking at the right cluster
gcloud config set project $project-$environ
gcloud config set compute/region $region
gcloud config set compute/zone $zone
gcloud container clusters get-credentials $container

# replace existing local image
docker rmi -f gcr.io/$project-$environ/$app:$version
docker build --build-arg port=$port --build-arg SSH_PRIVATE_KEY="$(cat ~/.ssh/id_rsa)" -t gcr.io/$project-$environ/$app:$version .

# replace existing remote image
gcloud container images delete -q --force-delete-tags gcr.io/$project-$environ/$app:$version
gcloud docker -- push gcr.io/$project-$environ/$app:$version

echo "Build complete, deploying with "$replicas" copies to "$environ"."

# redeploy
if [ $force = "true" ]
then
  kubectl delete service $app
  kubectl delete deployment $app
fi

cat deploy.yaml | \
sed -e "s/\${app}/"$app"/" \
 -e "s/\${environ}/"$environ"/" \
 -e "s/\${port}/"$port"/" \
 -e "s/\${project}/"$project"/" \
 -e "s/\${region}/"$region"/" \
 -e "s/\${version}/"$version"/" \
  | kubectl apply -f -
