# secret-script
Proof-of-concept python script for sychronization between kubernetes secret and gcloud secret manager.

## Prerequisites
- Create a gke cluster.

- [Create service account for app](https://cloud.google.com/docs/authentication/production#command-line)

- Grant required permissions to the service account `service-account-name`.

	- Permission to get clusters:

		    gcloud projects add-iam-policy-binding k8s-jkns-gke-soak --member "serviceAccount:<service-account-name>@k8s-jkns-gke-soak.iam.gserviceaccount.com" --role "roles/container.clusterViewer"
	- Permission to manage secrets:

		- Create a custom iam role `iam-role-id` with container.secrets.* permissions and add the role to service account `service-account-name`:
			- service-secret-role.yaml

				    title: Service Account Secret Role
				    description: secret managing role for service accounts
				    stage: GA
				    includedPermissions:
				    - container.secrets.create
				    - container.secrets.list
				    - container.secrets.get
				    - container.secrets.delete
			
			- Create a custom iam role

				    gcloud iam roles create <iam-role-id> --project=k8s-jkns-gke-soak --file=service-sevret-role.yaml

			- Add the role to service account `service-account-name`:

				    gcloud projects add-iam-policy-binding k8s-jkns-gke-soak --member "serviceAccount:<service-account-name>@k8s-jkns-gke-soak.iam.gserviceaccount.com" --role "roles/<iam-role-id>"

		- Or just add [Kubernetes Engine Developer] role to service account `service-account-name`:

			    gcloud projects add-iam-policy-binding k8s-jkns-gke-soak --member "serviceAccount:<service-account-name>@k8s-jkns-gke-soak.iam.gserviceaccount.com" --role "roles/container.developer"

- [Configure cluster access for kubectl](https://cloud.google.com/kubernetes-engine/docs/how-to/cluster-access-for-kubectl) (This is already done in the entrypoint.sh file in this project)

## Sample Case
This script syncs the secrets in k8s and gcloud sm with the same 'secret_id'.
A sample secret is provided as the following yaml file.

test-secret.yaml
```
account: "shanfu"
password: "1234"
```
When updating a secret `secret_id` in k8s, it accesses the secret in gcloud sm with the same secret_id, and updates it to the new value.
Vise versa.

## Usage
Run directly with command line:

```
# run on local terminal

# create secrets with secret_id in both k8s and gcloud sm
./secret-script.py create --secret_id=docker-secret --file=test-secret.yaml
# get secrets with secret_id in both k8s and gcloud sm
./secret-script.py get --secret_id=docker-secret
# update secret with secret_id in gcloud sm, then sync to k8s
./secret-script.py update --direction g2k --secret_id=docker-secret --file=new-secret.yaml
# update secret with secret_id in k8s, then sync to gcloud sm
./secret-script.py update --direction k2g --secret_id=docker-secret --file=test-secret.yaml
# delete secrets with secret_id in both k8s and gcloud sm
./secret-script.py delete --secret_id=docker-secret
```

Or run in a docker container:
```
# build docker image
docker build -t gcr.io/k8s-jkns-gke-soak/secret-script .
# run image in container
docker run --name secret-script-container --rm -it  gcr.io/k8s-jkns-gke-soak/secret-script
```