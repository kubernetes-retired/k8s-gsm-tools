# gcloud secret manager utilities
from absl import app
from absl import flags

FLAGS = flags.FLAGS

import os
import yaml

def gcloudCreateSecret(secret_id):
	os.system("gcloud secrets create " + secret_id +" --replication-policy=automatic")

def gcloudDeleteSecret(secret_id):
	os.system("gcloud secrets delete " + secret_id)

def gcloudAddSecrVersion(secret_id, file):
	os.system("gcloud secrets versions add " +  secret_id + " --data-file " + file)

def gcloudAccessSecretVersion(secret_id, version_id):
	secret = {}
	os.system("gcloud secrets versions access " + version_id + " --secret " + secret_id + " > " + "gcloud_"+secret_id+".yaml")
	with open("gcloud_"+secret_id+".yaml") as f:
		data = yaml.load_all(f, Loader=yaml.FullLoader)
		for d in data:
			secret = d
	out = 'gcloud_res.yaml'
	with open(out, 'w') as outfile:
		yaml.dump(secret, outfile, default_flow_style=False)
	print("Gcloud secret:")
	print(secret)
	return out
