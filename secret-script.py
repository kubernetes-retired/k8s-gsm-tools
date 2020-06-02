#!/usr/bin/env python3


from gcloud_utils import *
from k8s_utils import *
import argparse
import sys

def parse_args(args):
	parser = argparse.ArgumentParser()
	parser.add_argument('action', help='Options: create, get, update, or delete')
	parser.add_argument('--secret_id', help='The id of the secret')
	parser.add_argument('--file', help='The yaml file containing the secret')
	parser.add_argument('--direction', help='Options: k2g or g2k')
	return parser.parse_args(args)


def main(args):
	# create secrets <args.secret_id> with file <args.file>
	if args.action == 'create':
		if args.secret_id is None or args.file is None:
			sys.exit("requires '--secret_id' and '--file' arguments")
		gcloudCreateSecret(args.secret_id)
		gcloudAddSecrVersion(args.secret_id, args.file)
		k8sCreateSecret(args.secret_id, args.file)

	# get secrets <args.secret_id>
	elif args.action == 'get':
		if args.secret_id is None:
			sys.exit("requires '--secret_id' arguments")
		gcloudAccessSecretVersion(args.secret_id, "latest")
		k8sAccessSecret(args.secret_id)
		print("=============================")


	# delte secrets <args.secret_id>
	elif args.action == 'delete':
		if args.secret_id is None :
			sys.exit("requires '--secret_id' argument")
		gcloudDeleteSecret(args.secret_id)
		k8sDeleteSecret(args.secret_id)

	# update secrets <args.secret_id> with file <args.file> in a platform and sync to the other platform
	# k2g: update kubernetes secret first, then sync to gcloud sm
	# g2k: update gcloud sm secret first, then sync to kubernetes
	elif args.action == 'update':
		if args.secret_id is None or args.file is None:
			sys.exit("requires '--secret_id' and '--file' arguments")

		if args.direction == "k2g":
			print("Update k8s secret: ")
			k8sUpdateSecret(args.secret_id, args.file)
			new_secret = k8sAccessSecret(args.secret_id)
			gcloudAccessSecretVersion(args.secret_id, "latest")
			
			# sync
			print("\n Synchronizing...\n")
			gcloudAddSecrVersion(args.secret_id, new_secret)
		elif args.direction == "g2k":
			print("Update gcloud secret: ")
			gcloudAddSecrVersion(args.secret_id, args.file)
			new_secret = gcloudAccessSecretVersion(args.secret_id, "latest")
			k8sAccessSecret(args.secret_id)
			
			# sync
			print("\n Synchronizing...\n")
			k8sUpdateSecret(args.secret_id, new_secret)
		else:
			sys.exit("missing or invalid '--direction' argument, options: k2g or g2k")

		gcloudAccessSecretVersion(args.secret_id, "latest")
		k8sAccessSecret(args.secret_id)
		print("Sync'ed.")
		print("=============================\n")

	else:
		sys.exit("missing or invalid 'action' argument, options: create, get, update, or delete")
		


if __name__ == '__main__':
	main(parse_args(sys.argv[1:]))
