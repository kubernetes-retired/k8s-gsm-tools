# k8s-gsm-tools

Secret rotation and synchronization integrating Google Cloud Secret Manager and Kubernetes.

## Current Functions
Parse configuration file into source and destination secrets.

Fetch the latest versions of from Secret Manager secret and Kubernetes secrets.

## Prerequisites
- Create a gke cluster.

- [Create service account for app](https://cloud.google.com/docs/authentication/production#command-line)

- Enable Identity and Access Management (IAM) API for project.

		gcloud services enable iam.googleapis.com --project=<gcloud-project-id>

- Grant required permissions to the service account `gsa-name`.
	
	- Permission to manage service account keys:

		    gcloud projects add-iam-policy-binding <gcloud-project-id> --member "serviceAccount:<gsa-name>@<gcloud-project-id>.iam.gserviceaccount.com" --role "roles/iam.serviceAccountKeyAdmin"

	- Permission to get clusters:

		    gcloud projects add-iam-policy-binding <gcloud-project-id> --member "serviceAccount:<gsa-name>@<gcloud-project-id>.iam.gserviceaccount.com" --role "roles/container.clusterViewer"
	
	- Permission to manage secrets:

		    gcloud projects add-iam-policy-binding <gcloud-project-id> --member "serviceAccount:<gsa-name>@<gcloud-project-id>.iam.gserviceaccount.com" --role "roles/secretmanager.admin"

	- Permission to manage secrets within containers:

		- Create a custom iam role `iam-role-id` with container.secrets.* permissions and add the role to service account `gsa-name`:
			- service-secret-role.yaml

				    title: Kubernetes Engine Secret Admin
				    description: Provides access to management of Kubernetes Secrets
				    stage: GA
				    includedPermissions:
				    - container.secrets.create
				    - container.secrets.list
				    - container.secrets.get
				    - container.secrets.delete
				    - container.secrets.update
			
			- Create a custom iam role

				    gcloud iam roles create <iam-role-id> --project=<gcloud-project-id> --file=service-secret-role.yaml

			- Add the role to service account `gsa-name`:

				    gcloud projects add-iam-policy-binding <gcloud-project-id> --member "serviceAccount:<gsa-name>@<gcloud-project-id>.iam.gserviceaccount.com" --role "roles/<iam-role-id>"

		- Or just add [Kubernetes Engine Developer] role to service account `gsa-name`:

			    gcloud projects add-iam-policy-binding <gcloud-project-id> --member "serviceAccount:<gsa-name>@<gcloud-project-id>.iam.gserviceaccount.com" --role "roles/container.developer"

- Modify the cluster to enable Workload Identity
```
gcloud container clusters update <cluster-name> \
  --workload-pool=<gcloud-project-id>.svc.id.goog
```

- Modify an existing node pool to enable GKE_METADATA
```
gcloud container node-pools update <nodepool-name> \
  --cluster=<cluster-name> \
  --workload-metadata=GKE_METADATA
```

- Create Kubernetes service account
```
kubectl apply -f service-account/serviceaccount.yaml
```

- Set up Workload Identity binding
```
gcloud iam service-accounts add-iam-policy-binding \
  --role roles/iam.workloadIdentityUser \
  --member "serviceAccount:<gcloud-project-id>.svc.id.goog[<k8s_namespace>/<ksa_name>]" \
  <gsa-name>@<gcloud-project-id>.iam.gserviceaccount.com
```

- Annotate the KSA to complete the binding between the KSA and GSA
```
kubectl annotate serviceaccount \
  --namespace <k8s_namespace> \
   <ksa_name> \
   iam.gke.io/gcp-service-account=<gsa-name>@<gcloud-project-id>.iam.gserviceaccount.com
```

- Set up Kubernetes service account role and binding
(action might require container.roles.create and container.roles.bind permissions if using gke cluster)
```
kubectl apply -f service-account/role.yaml
```

## Usage
- secret-sync-controller	
	- create ConfigMap `config` with key `syncConfig`.

	- deploy controller in continuous mode
			
			kubectl apply -f cmd/secret-sync-controller/deployment.yaml

	- run testing job

			kubectl apply -f cmd/secret-sync-controller/test-job.yaml
	
- secret-rotator	
	- create ConfigMap `config` with key `rotConfig`.

	- deploy rotator in continuous mode
			
			kubectl apply -f cmd/secret-rotator/deployment.yaml

- test-svc-consumer	
	- build image locally and push

			docker build --pull \
			--build-arg "cmd=consumer" \
			-t "gcr.io/<gcloud-project-id>/consumer:latest" \
			-f "./images/default/Dockerfile" .
			docker push gcr.io/<gcloud-project-id>/consumer

	- run consumer as a job
			
			kubectl apply -f experiment/cmd/consumer/job.yaml


## Demo for rotating service account keys
- create Secret Manager secret and Kubernetes namespace

		gcloud secrets create secret-1
		kubectl create namespace ns-a 

- deploy secret-sync-controller

		kubectl apply -f cmd/secret-sync-controller/deployment.yaml

- deploy secret-rotator

		kubectl apply -f cmd/secret-rotator/deployment.yaml

- deploy svc-consumer	

		kubectl apply -f experiment/cmd/consumer/job.yaml

- get logs
		
		kubectl logs -n ns-a <svc-consumer-pod>

- cleanup

		kubectl delete -f cmd/secret-sync-controller/deployment.yaml
		kubectl delete -f cmd/secret-rotator/deployment.yaml
		kubectl delete -f experiment/cmd/consumer/job.yaml
		gcloud secrets delete secret-1
		kubectl create namespace ns-a 
		

## Building / pushing images

To build images locally:

    make images

If you have access to a GCP project that has Google Cloud Build enabled:

    gcloud builds submit --config=./images/cloudbuild.yaml .

This file can be used by a [prow image-pushing job][image-pushing-readme] to push to the project's repository

[image-pushing-readme]: https://github.com/kubernetes/test-infra/blob/master/config/jobs/image-pushing/README.md
