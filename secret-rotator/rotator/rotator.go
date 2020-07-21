/*
Copyright 2020 The Kubernetes Authors.
Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at
    http://www.apache.org/licenses/LICENSE-2.0
Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package rotator

import (
	"k8s.io/klog"
	"regexp"
	"sigs.k8s.io/k8s-gsm-tools/secret-rotator/client"
	"sigs.k8s.io/k8s-gsm-tools/secret-rotator/config"
	"strconv"
	"strings"
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SecretProvisioner interface {
	CreateNew(labels map[string]string) (string, string, error)
	Deactivate(labels map[string]string, version string) error
}

type SecretRotator struct {
	Client       client.Interface
	Agent        *config.Agent
	Provisioners map[string]SecretProvisioner
}

// RunOnce checks all rotated secrets in Agent.Config().Specs
// Pops error message for any failure in refreshing or deactivating each secret.
func (r *SecretRotator) RunOnce() {
	// iterate on copy of Specs instead of index,
	// so that the update in Agent.config will only be observed outside of the loop
	for _, rotatedSecret := range r.Agent.Config().Specs {
		_, err := r.Refresh(rotatedSecret, time.Now())
		if err != nil {
			klog.Error(err)
		}

		err = r.Deactivate(rotatedSecret, time.Now())
		if err != nil {
			klog.Error(err)
		}
	}
}

// Refresh checks whether the secret needs to be refreshed according to 'now' and 'rotatedSecret.Refresh'.
// If it does, Refresh provisions a new secret and updates the Secret Manager secret.
// Returns true if the secret is refreshed.
func (r *SecretRotator) Refresh(rotatedSecret config.RotatedSecretSpec, now time.Time) (bool, error) {
	createTime, err := r.Client.GetCreateTime(rotatedSecret.Project, rotatedSecret.Secret, "latest")
	if err != nil {
		return false, err
	}

	labels, err := r.Client.GetSecretLabels(rotatedSecret.Project, rotatedSecret.Secret)
	if err != nil {
		return false, err
	}

	if rotatedSecret.Refresh.Interval != 0 {
		// the refresh stratetgy is refreshInterval
		// check the elapsed time from its createTime to now.
		if now.After(createTime.Add(rotatedSecret.Refresh.Interval)) {
			// provision a new secret
			newId, newSecret, err := r.Provisioners[rotatedSecret.Type.Type()].CreateNew(labels)
			if err != nil {
				return false, err
			}

			// update the secret Manager secret
			versionName, err := r.Client.UpsertSecret(rotatedSecret.Project, rotatedSecret.Secret, []byte(newSecret))
			if err != nil {
				return false, err
			}

			// extract the latest version number instead of storing the entire versionName
			// because '/'s are not allowed in gsm metedat
			splits := strings.Split(versionName, "/")
			latestVersion := splits[len(splits)-1]
			err = r.Client.UpsertSecretLabel(rotatedSecret.Project, rotatedSecret.Secret, "v"+latestVersion, newId)
			if err != nil {
				return false, err
			}

			return true, nil
		}
	} else {
		// the refresh strategy is cron
		// TODO
	}

	return false, nil
}

// Deactivate checks the old secret versions that are recorded in the secret labels,
// if any needs to be deactivated according to 'now' and 'rotatedSecret.GracePeriod',
// Deactivate deactivates that old version and updates the Secret Manager secret.
func (r *SecretRotator) Deactivate(rotatedSecret config.RotatedSecretSpec, now time.Time) error {
	labels, err := r.Client.GetSecretLabels(rotatedSecret.Project, rotatedSecret.Secret)
	if err != nil {
		return err
	}

	for key, _ := range labels {
		// the key of a label for [version: data] pair should be in the format of "v%d"
		matched, err := regexp.Match(`^v[0-9]+$`, []byte(key))
		if err != nil {
			return err
		}

		if !matched {
			continue
		}

		// for each version, check the elapsed time from its next version's createTime to now.
		curVersion := key[1:]
		v, _ := strconv.Atoi(curVersion)
		nextVersion := strconv.Itoa(v + 1)

		err = r.Client.ValidateSecretVersion(rotatedSecret.Project, rotatedSecret.Secret, curVersion)
		if err != nil {
			return err
		}

		err = r.Client.ValidateSecretVersion(rotatedSecret.Project, rotatedSecret.Secret, nextVersion)
		if err != nil {
			// if nextVersion does not exist, then curVersion is the latest. Do not return err.
			if status.Code(err) == codes.NotFound {
				continue
			} else {
				return err
			}
		}

		nextCreateTime, err := r.Client.GetCreateTime(rotatedSecret.Project, rotatedSecret.Secret, nextVersion)
		if err != nil {
			return err
		}

		if now.After(nextCreateTime.Add(rotatedSecret.GracePeriod)) {
			// deactivate the old secret
			err := r.Provisioners[rotatedSecret.Type.Type()].Deactivate(labels, curVersion)
			if err != nil {
				return err
			}

			// update the Secret Manager secret
			err = r.Client.DeleteSecretLabel(rotatedSecret.Project, rotatedSecret.Secret, "v"+curVersion)
			if err != nil {
				return err
			}
		}

		// TODO: disable or destroy Secret Manager secret version curVersion
	}

	return nil
}
