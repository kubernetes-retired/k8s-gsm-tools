from absl import app
from absl import flags

FLAGS = flags.FLAGS

flags.DEFINE_boolean('k2g', False, 'Secret sync from Kubernetes to Gcloud')
flags.DEFINE_boolean('g2k', False, 'Secret sync from Kubernetes to Gcloud')
flags.DEFINE_boolean('create', None, 'Create new secret with secret_id')
flags.DEFINE_boolean('delete', None, 'Delete secret with secret_id')
flags.DEFINE_boolean('get', None, 'Get secret with secret_id')
flags.DEFINE_string('file', None, 'Create or update from file')
flags.DEFINE_string('secret_id', None, 'The string id of the secret')

flags.mark_flag_as_required("secret_id")

from gcloud_utils import *
from k8s_utils import *


def main(argv):
	if len(argv) > 1:
		raise app.UsageError('Too many command-line arguments.')

	# create secrets with id = FLAGS.secret_id
	if FLAGS.create is not None:
		gcloudCreateSecret(FLAGS.secret_id)
		gcloudAddSecrVersion(FLAGS.secret_id, FLAGS.file)
		k8sCreateSecret(FLAGS.secret_id, FLAGS.file)

	# get secrets with id = FLAGS.secret_id
	elif FLAGS.get is not None:
		gcloudAccessSecretVersion(FLAGS.secret_id, "latest")
		k8sAccessSecret(FLAGS.secret_id)
		print("=============================")


	# delte secrets with id = FLAGS.secret_id
	elif FLAGS.delete is not None:
		gcloudDeleteSecret(FLAGS.secret_id)
		k8sDeleteSecret(FLAGS.secret_id)

	# secret update in k8s, then sync to gcloud SM
	elif FLAGS.k2g:
		print("Update k8s secret: ")
		k8sUpdateSecret(FLAGS.secret_id, FLAGS.file)
		new_secret = k8sAccessSecret(FLAGS.secret_id)
		gcloudAccessSecretVersion(FLAGS.secret_id, "latest")
		
		# sync
		sync = input("Sync? [y/n] ")
		if sync in ['n','N']:
			return

		print("Synchronizing...")
		gcloudAddSecrVersion(FLAGS.secret_id, new_secret)
		gcloudAccessSecretVersion(FLAGS.secret_id, "latest")
		k8sAccessSecret(FLAGS.secret_id)
		print("Sync'ed.")
		print("=============================")

	# secret update in gcloud SM, then sync to k8s
	elif FLAGS.g2k:
		print("Update gcloud secret: ")
		gcloudAddSecrVersion(FLAGS.secret_id, FLAGS.file)
		new_secret = gcloudAccessSecretVersion(FLAGS.secret_id, "latest")
		k8sAccessSecret(FLAGS.secret_id)
		
		# sync
		sync = input("Sync? [y/n] ")
		if sync in ['n','N']:
			return

		print("Synchronizing...")
		k8sUpdateSecret(FLAGS.secret_id, new_secret)
		k8sAccessSecret(FLAGS.secret_id)
		gcloudAccessSecretVersion(FLAGS.secret_id, "latest")
		print("Sync'ed.")
		print("=============================")
		


if __name__ == '__main__':
	app.run(main)
