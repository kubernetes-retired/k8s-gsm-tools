# Lint as: python3
"""TODO(shanefu): DO NOT SUBMIT without one-line documentation for utils.

TODO(shanefu): DO NOT SUBMIT without a detailed description of utils.
"""
'''
from __future__ import absolute_import
from __future__ import division
from __future__ import google_type_annotations
from __future__ import print_function
'''

from absl import app
from absl import flags

FLAGS = flags.FLAGS

import os
import yaml

def gcloudCreateSecret(secret_id):
	os.system("gcloud secrets create " + secret_id +" --replication-policy=automatic")

def gcloudAddSecrVersion(secret_id, file):
	os.system("gcloud secrets versions add " +  secret_id + " --data-file " + file)

def gcloudAccessSecretVersion(secret_id, version_id):
	secret = {}
	os.system("gcloud secrets versions access " + version_id + " --secret " + secret_id + " > " + "gcloud_"+secret_id+".yaml")
	with open("gcloud_"+secret_id+".yaml") as f:
		data = yaml.load_all(f, Loader=yaml.FullLoader)
		for d in data:
			secret = d
	out = 'gcloud_res.yml'
	with open(out, 'w') as outfile:
		yaml.dump(secret, outfile, default_flow_style=False)
	print("Gcloud secret:")
	print(secret)
	return out
