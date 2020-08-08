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

package controller

import (
	"bytes"
	"k8s.io/klog"
	"sigs.k8s.io/k8s-gsm-tools/secret-sync-controller/client"
	"sigs.k8s.io/k8s-gsm-tools/secret-sync-controller/config"
	"time"
)

type SecretSyncController struct {
	Client       client.Interface
	Agent        *config.Agent
	RunOnce      bool
	ResyncPeriod time.Duration
}

// Start starts the secret sync controller in continuous mode.
// stops when stop sinal is received from stopChan.
func (c *SecretSyncController) Start(stopChan <-chan struct{}) error {
	runChan := make(chan struct{})

	go func() {
		for {
			runChan <- struct{}{}
			time.Sleep(c.ResyncPeriod)
		}
	}()

	for {
		select {
		case <-stopChan:
			klog.V(2).Info("Stop signal received. Quitting...")
			return nil
		case <-runChan:
			c.SyncAll()
			if c.RunOnce {
				return nil
			}
		}
	}
}

// SyncAll sychronizes all secret pairs specified in Agent.Config().Specs
// Pops error message for any secret pair that it failed to sync or access
func (c *SecretSyncController) SyncAll() {
	// iterate on copy of Specs instead of index,
	// so that the update in Agent.config will only be observed outside of the loop SyncAll()
	for _, spec := range c.Agent.Config().Specs {
		updated, err := c.Sync(spec)
		if err != nil {
			klog.Errorf("Secret sync failed for %s: %s", spec, err)
		}
		if updated {
			klog.V(2).Infof("Secret %s synced from %s", spec.Destination, spec.Source)
		}
	}
}

// Sync sychronizes the secret value from spec.Source to spec.Destination.
// Returns true if the secret value in spec.Destination is updated,
// otherwise returns false, meaning that the secret value in spec.Destination remains unchanged.
func (c *SecretSyncController) Sync(spec config.SecretSyncSpec) (bool, error) {
	// get source secret
	srcData, err := c.Client.GetSecretManagerSecretValue(spec.Source.Project, spec.Source.Secret)
	if err != nil {
		return false, err
	}

	// get destination secret
	destData, err := c.Client.GetKubernetesSecretValue(spec.Destination.Namespace, spec.Destination.Secret, spec.Destination.Key)
	if err != nil {
		return false, err
	}

	// update destination secret
	if bytes.Equal(srcData, destData) {
		return false, nil
	}
	// update destination secret value
	// inserts a key-value pair if spec.Destination does not exist yet
	err = c.Client.UpsertKubernetesSecret(spec.Destination.Namespace, spec.Destination.Secret, spec.Destination.Key, srcData)
	if err != nil {
		return false, err
	}

	return true, nil
}
