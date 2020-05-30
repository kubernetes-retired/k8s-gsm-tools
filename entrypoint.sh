#!/bin/bash

export GOOGLE_APPLICATION_CREDENTIALS="/usr/src/app/gcloud_key.json"
gcloud config set project k8s-jkns-gke-soak
gcloud config set account shanefu@k8s-jkns-gke-soak.iam.gserviceaccount.com
gcloud config set compute/zone us-central1-f
gcloud auth activate-service-account --key-file $GOOGLE_APPLICATION_CREDENTIALS
gcloud container clusters get-credentials shanefu


kubectl get secrets

python secret-script.py --create --secret_id=docker-secret --file=test-secret.yaml
python secret-script.py --get --secret_id=docker-secret
python secret-script.py --g2k --secret_id=docker-secret --file=new-secret.yaml
python secret-script.py --k2g --secret_id=docker-secret --file=test-secret.yaml



