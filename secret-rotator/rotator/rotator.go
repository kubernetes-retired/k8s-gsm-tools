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
	"time"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
)

type SecretProvisioner interface {
	CreateNew(labels map[string]string) (string, []byte, error)
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
	// iterating on rotatedSecret instead of index so that the config stays consistent within each iteration,
	// even if a config update occurs in the middle of the loop.
	for _, rotatedSecret := range r.Agent.Config().Specs {
		err := r.UpsertLabels(rotatedSecret)
		if err != nil {
			klog.Error(err)
		}

		_, err = r.Refresh(rotatedSecret, time.Now())
		if err != nil {
			klog.Error(err)
		}

		err = r.Deactivate(rotatedSecret, time.Now())
		if err != nil {
			klog.Error(err)
		}
	}
}

// UpsertLabels updates or inserts labels needed by the provisioner specified by rotatedSecret
// Returns error if fails.
func (r *SecretRotator) UpsertLabels(rotatedSecret config.RotatedSecretSpec) error {
	_, err := r.Client.GetSecretLabels(rotatedSecret.Project, rotatedSecret.Secret)
	if err != nil {
		return err
	}

	// attach the labels needed for the provisioner
	for key, val := range rotatedSecret.Type.Labels() {
		err = r.Client.UpsertSecretLabel(rotatedSecret.Project, rotatedSecret.Secret, key, val)
		if err != nil {
			return err
		}
	}

	return nil
}

// Refresh checks if the secret needs to be refreshed, and if so
// provisions a new secret and updates the Secret Manager secret.
// Returns true if the secret is refreshed.
func (r *SecretRotator) Refresh(rotatedSecret config.RotatedSecretSpec, now time.Time) (bool, error) {
	shouldRefresh, err := r.ShouldRefresh(rotatedSecret, now)
	if err != nil {
		return false, err
	}

	if !shouldRefresh {
		return false, nil
	}

	labels, err := r.Client.GetSecretLabels(rotatedSecret.Project, rotatedSecret.Secret)
	if err != nil {
		return false, err
	}

	if labels == nil {
		labels = make(map[string]string)
	}

	// attach the labels needed for the provisioner
	for key, val := range rotatedSecret.Type.Labels() {
		labels[key] = val
	}

	newId, newSecret, err := r.Provisioners[rotatedSecret.Type.Type()].CreateNew(labels)
	if err != nil {
		return false, err
	}

	// update the secret Manager secret
	latestVersion, err := r.Client.UpsertSecret(rotatedSecret.Project, rotatedSecret.Secret, newSecret)
	if err != nil {
		return false, err
	}

	// keys in format of "v%d" indicate that they are (version: id) pairs attached by the rotator
	// the reason for the prefix "v" is that Secret Manager labels need to begin with a lowwer case letter
	err = r.Client.UpsertSecretLabel(rotatedSecret.Project, rotatedSecret.Secret, "v"+latestVersion, newId)
	if err != nil {
		return false, err
	}

	return true, nil

}

// ShouldRefresh checks whether the secret needs to be refreshed according to 'now' and 'rotatedSecret.Refresh'.
// Returns true if the secret needs to be refreshed.
func (r *SecretRotator) ShouldRefresh(rotatedSecret config.RotatedSecretSpec, now time.Time) (bool, error) {
	err := r.Client.ValidateSecretVersion(rotatedSecret.Project, rotatedSecret.Secret, "1")
	if err != nil {
		// create the secret and/or dirst version if it does not already exist
		if status.Code(err) == codes.NotFound {
			return true, nil
		}
		return false, err
	}

	createTime, err := r.Client.GetCreateTime(rotatedSecret.Project, rotatedSecret.Secret, "latest")
	if err != nil {
		return false, err
	}

	if rotatedSecret.Refresh.Interval != 0 {
		// the refresh stratetgy is refreshInterval
		// check the elapsed time from its createTime to now.
		if now.After(createTime.Add(rotatedSecret.Refresh.Interval)) {
			return true, nil
		}
	} else {
		// the refresh strategy is cron
		// TODO
	}

	return false, nil
}

// Deactivate fetches the secret versions from the Secret Manager secret labels,
// if any version needs to be deactivated, deactivates it and updates the Secret Manager secret accordingly.
func (r *SecretRotator) Deactivate(rotatedSecret config.RotatedSecretSpec, now time.Time) error {
	labels, err := r.Client.GetSecretLabels(rotatedSecret.Project, rotatedSecret.Secret)
	if err != nil {
		return err
	}

	if labels == nil {
		labels = make(map[string]string)
	}

	// attach the labels needed for the provisioner
	for key, val := range rotatedSecret.Type.Labels() {
		labels[key] = val
	}

	for key, _ := range labels {
		// keys in format of "v%d" indicate that they are (version: id) pairs attached by the rotator
		// the reason for the prefix "v" is that Secret Manager labels need to begin with a lowwer case letter
		matched, err := regexp.Match(`^v[0-9]+$`, []byte(key))
		if err != nil {
			klog.Errorf("Fail to label %s in %s: %s", key, rotatedSecret, err)
			continue
		}

		if !matched {
			continue
		}

		version := key[1:]

		shouldDeactivate, err := r.ShouldDeactivate(rotatedSecret, version, now)
		if err != nil {
			klog.Errorf("Fail to check for deactivating %s/%s: %s", rotatedSecret, version, err)
		}

		if !shouldDeactivate {
			continue
		}

		err = r.Provisioners[rotatedSecret.Type.Type()].Deactivate(labels, version)
		if err != nil {
			klog.Errorf("Fail to deactivate %s/%s: %s", rotatedSecret, version, err)
			continue
		}

		// destroy the Secret Manager secret version after the provision deactivates
		err = r.Client.DestroySecretVersion(rotatedSecret.Project, rotatedSecret.Secret, version)
		if err != nil {
			klog.Errorf("Fail to disable %s/%s: %s", rotatedSecret, version, err)
			continue
		}

		// update the Secret Manager secret
		// keys in format of "v%d" indicate that they are (version: id) pairs attached by the rotator
		// the reason for the prefix "v" is that Secret Manager labels need to begin with a lowwer case letter
		err = r.Client.DeleteSecretLabel(rotatedSecret.Project, rotatedSecret.Secret, "v"+version)
		if err != nil {
			klog.Errorf("Fail to delete label %s of %s: %s", "v"+version, rotatedSecret, err)
			continue
		}
	}

	return nil
}

// ShouldDeactivate checks if the secret version needs to be deactivated according to 'now' and 'rotatedSecret.GracePeriod'
// Returns true if the secret version needs to be deactivated.
func (r *SecretRotator) ShouldDeactivate(rotatedSecret config.RotatedSecretSpec, version string, now time.Time) (bool, error) {

	// check the elapsed time from its next version's createTime to now.
	v, _ := strconv.Atoi(version)
	nextVersion := strconv.Itoa(v + 1)

	// check if version exists
	err := r.Client.ValidateSecretVersion(rotatedSecret.Project, rotatedSecret.Secret, version)
	if err != nil {
		return false, err
	}

	// check if nextVersion exists
	err = r.Client.ValidateSecretVersion(rotatedSecret.Project, rotatedSecret.Secret, nextVersion)
	if err != nil {
		// if nextVersion does not exist, then version is the latest. Return false to signal no deactivation.
		if status.Code(err) == codes.NotFound {
			return false, nil
		} else {
			return false, err
		}
	}

	nextCreateTime, err := r.Client.GetCreateTime(rotatedSecret.Project, rotatedSecret.Secret, nextVersion)
	if err != nil {
		return false, err
	}

	if now.After(nextCreateTime.Add(rotatedSecret.GracePeriod)) {
		return true, nil
	}

	return false, nil
}
