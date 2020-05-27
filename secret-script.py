# Lint as: python3
"""TODO(shanefu): DO NOT SUBMIT without one-line documentation for secret-script.

TODO(shanefu): DO NOT SUBMIT without a detailed description of secret-script.
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

flags.DEFINE_boolean('k2g', False, 'Secret sync from Kubernetes to Gcloud')
flags.DEFINE_boolean('g2k', False, 'Secret sync from Kubernetes to Gcloud')
flags.DEFINE_boolean('create', None, 'Create new secrets')
flags.DEFINE_string('file', None, 'Create or update from file')
flags.DEFINE_string('secret_id', None, 'The string id of the secret')

flags.mark_flag_as_required("secret_id")
flags.mark_flag_as_required("file")

from gcloud_utils import *
from k8s_utils import *


def main(argv):
	if len(argv) > 1:
		raise app.UsageError('Too many command-line arguments.')

	if FLAGS.create is not None:
		gcloudCreateSecret(FLAGS.secret_id)
		gcloudAddSecrVersion(FLAGS.secret_id, FLAGS.file)
		k8sCreateSecret(FLAGS.secret_id, FLAGS.file)

	elif FLAGS.k2g:
		k8sUpdateSecret(FLAGS.secret_id, FLAGS.file)
		new_secret = k8sAccessSecret(FLAGS.secret_id)
		gcloudAddSecrVersion(FLAGS.secret_id, new_secret)
		gcloudAccessSecretVersion(FLAGS.secret_id, "latest")

	elif FLAGS.g2k:
		gcloudAddSecrVersion(FLAGS.secret_id, FLAGS.file)
		new_secret = gcloudAccessSecretVersion(FLAGS.secret_id, "latest")
		k8sUpdateSecret(FLAGS.secret_id, new_secret)
		k8sAccessSecret(FLAGS.secret_id)
		


if __name__ == '__main__':
	app.run(main)
