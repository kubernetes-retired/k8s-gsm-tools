# secret-script
Proof-of-concept python script for sychronization between kubernetes secret and gcloud secret manager.

## Prerequisites
Create a gke cluster.

[Create service account for app](https://cloud.google.com/docs/authentication/production#command-line)

[Configure cluster access for kubectl](https://cloud.google.com/kubernetes-engine/docs/how-to/cluster-access-for-kubectl)

## Sample Case
This script syncs the secrets in k8s and gcloud sm with the same 'secret_id'.
A sample secret is provided as the following yaml file.

test-secret.yaml
```
account: "shanfu"
password: "1234"
```
When updating a secret<secret_id> in k8s, it accesses the secret in gcloud sm with the same secret_id, and updates it to the new value.
Vise versa.

## Usage
Run directly with command line:

```
# run on local terminal

# create secrets with secret_id in both k8s and gcloud sm
python secret-script.py --create --secret_id=secret_id --file=test-secret.yaml
# get secrets with secret_id in both k8s and gcloud sm
python secret-script.py --get --secret_id=secret_id
# update secret with secret_id in gcloud sm, then sync to k8s
python secret-script.py --g2k --secret_id=secret_id --file=new-secret.yaml
# update secret with secret_id in k8s, then sync to gcloud sm
python secret-script.py --k2g --secret_id=secret_id --file=test-secret.yaml
```

Or run in a docker container:
```
# build docker image
docker build -t python-app .
# run image in container
docker run --name my-running-app --rm -it  python-app
```