# secret_sync
Synchronizing between Google Cloud Secret Manager secrets and Kubernetes secrets.

## Current Functions
Parse configuration file into source and destination secrets.

Fetch the latest versions of from Secret Manager secret and Kubernetes secrets.

## Prerequisites
- Create a gke cluster.

- [Create service account for app](https://cloud.google.com/docs/authentication/production#command-line)

- Grant required permissions to the service account `gsa-name`.

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
- Out-of-Cluster
	- testing with mock-client

			go test -v ./...

	- testing pkg/controller with e2e-client
	
			cd pkg/controller/
			go test -v --e2e-client --gsm-project <gcloud-project-id>
	
	- build binary

			go build -o secret-sync-controller cmd/secret-sync-controller/main.go
			# mock mounted ConfigMap
			mkdir ..data

	- run controller in continuous mode
	
			./secret-sync-controller --config-path config.yaml --period 1000 
	
	- run controller in one-shot mode

			./secret-sync-controller --config-path config.yaml --period 1000 --run-once

	- run controller demo

			cd cmd/demo
			go build
			# to mock mounted ConfigMap 
			mkdir ..data
			./demo --config-path config.yaml \
            --resync-period 1000 \
            --poll-period 500 \
            --duration 70000 \
            --switch-period 10000 \
            --gsm-project=k8s-jkns-gke-soak \
            --output-path .
			
	
- In-Cluster
	- build and push docker image

			docker build -t gcr.io/k8s-jkns-gke-soak/secret-sync-controller .
			docker push gcr.io/k8s-jkns-gke-soak/secret-sync-controller

	- create ConfigMap `syncConfig` and mount it to `/tmp/config`.

	- run controller in continuous mode as job
			
			kubectl apply -f controller-job.yaml

	- run testing job

			kubectl apply -f test-controller-job.yaml
	
	- run controller demo

			kubectl apply -f cmd/demo/demo-job.yaml
		



## Building / pushing images

To build images locally:

    make images

If you have access to a GCP project that has Google Cloud Build enabled:

    gcloud builds submit --config=./images/cloudbuild.yaml .

This file can be used by a [prow image-pushing job][image-pushing-readme] to push to the project's repository

[image-pushing-readme]: https://github.com/kubernetes/test-infra/blob/master/config/jobs/image-pushing/README.md
