# k8s secret manager utilities
from absl import app
from absl import flags

FLAGS = flags.FLAGS

import os
import yaml
import subprocess
import base64

def k8sCreateSecret(secret_id, file):
	literal = ""
	with open(file) as f:
		data = yaml.load_all(f, Loader=yaml.FullLoader)
		for d in data:
			for k,v in d.items():
				literal += " --from-literal=" + str(k) +"=" + str(v)
	os.system("kubectl create secret generic " + secret_id + literal)

def k8sDeleteSecret(secret_id):
	os.system("kubectl delete secret " + secret_id)

def k8sUpdateSecret(secret_id, file):
	literal = ""
	with open(file) as f:
		data = yaml.load_all(f, Loader=yaml.FullLoader)
		for d in data:
			for k,v in d.items():
				literal += " --from-literal=" + str(k) +"=" + str(v)
	os.system("kubectl create secret generic " + secret_id + literal + " --dry-run=client -o yaml | kubectl apply -f -")


def k8sAccessSecret(secret_id):
	secret = {}
	os.system("kubectl get secret " + secret_id + " -o yaml > " + "k8s_"+secret_id+".yaml")
	with open("k8s_"+secret_id+".yaml") as f:
		data = yaml.load_all(f, Loader=yaml.FullLoader)

		for d in data:
			for k,v in d.items():
				if k!='data': continue
				for k_,s_ in v.items():
					secret[k_] = base64.b64decode(s_).decode("utf-8")
	out = 'k8s_res.yaml'
	with open(out, 'w') as outfile:
		yaml.dump(secret, outfile, default_flow_style=False)
	print("K8s secret:")
	print(secret)
	return out
