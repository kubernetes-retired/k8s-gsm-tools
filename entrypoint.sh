#!/bin/bash

GCP_PROJECT="${GCP_PROJECT:-k8s-jkns-gke-soak}"
GCP_ZONE="${GCP_ZONE:-us-central1-f}"
GCP_CLUSTER="${GCP_CLUSTER:-shanefu}"

gcloud config set project "${GCP_PROJECT}"
gcloud config set compute/zone "${GCP_ZONE}"
gcloud auth activate-service-account --key-file "${GOOGLE_APPLICATION_CREDENTIALS}"
gcloud container clusters get-credentials "${GCP_CLUSTER}"


kubectl get secrets

chmod +x secret-script.py

./secret-script.py --create --secret_id=docker-secret --file=test-secret.yaml
./secret-script.py --get --secret_id=docker-secret
./secret-script.py --g2k --secret_id=docker-secret --file=new-secret.yaml
./secret-script.py --k2g --secret_id=docker-secret --file=test-secret.yaml



