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

package config

import (
	"fmt"
	"sync"

	cron "gopkg.in/robfig/cron.v2"
	utilerrors "k8s.io/apimachinery/pkg/util/errors"
	"k8s.io/apimachinery/pkg/util/sets"
)

// Cron is a wrapper for cron.Cron
// It is responsible for refreshing rotated secrets with cron as refreshStrategy
type Cron struct {
	cronAgent *cron.Cron
	secrets   map[string]*secretStatus
	lock      sync.Mutex
}

// secretStatus is a cache layer for tracking existing cron for secret-refresh
type secretStatus struct {
	// entryID is a unique-identifier for each cron entry generated from cronAgent
	entryID cron.EntryID
	// triggered marks if a secret-refresh has been triggered for the next cron.QueuedSecrets() call
	triggered bool
	// cronStr is a cache for secret-refresh's cron status
	// cron entry will be regenerated if cron string changes from the config
	cronStr string
}

// NewCron makes a new Cron object
func NewCron() *Cron {
	return &Cron{
		cronAgent: cron.New(),
		secrets:   map[string]*secretStatus{},
	}
}

// Start kicks off current cronAgent scheduler
func (c *Cron) Start() {
	c.cronAgent.Start()
}

// Stop pauses current cronAgent scheduler
func (c *Cron) Stop() {
	c.cronAgent.Stop()
}

// QueuedSecrets returns a set of secret names that need to be triggered
// and resets trigger in secretStatus
func (c *Cron) QueuedSecrets() sets.String {
	c.lock.Lock()
	defer c.lock.Unlock()

	res := sets.NewString()
	for k, v := range c.secrets {
		if v.triggered {
			res.Insert(k)
		}
		c.secrets[k].triggered = false
	}
	return res
}

// SyncConfig syncs current cronAgent with input rotation config
// which adds/deletes secret-refresh crons accordingly.
func (c *Cron) SyncConfig(cfg *RotatedSecretConfig) error {
	c.lock.Lock()
	defer c.lock.Unlock()

	for _, spec := range cfg.Specs {
		if err := c.addPeriodic(spec); err != nil {
			return err
		}
	}

	periodicNames := sets.NewString()
	for _, spec := range cfg.Specs {
		periodicNames.Insert(spec.String())
	}

	existing := sets.NewString()
	for k := range c.secrets {
		existing.Insert(k)
	}

	var removalErrors []error
	for _, secret := range existing.Difference(periodicNames).List() {
		if err := c.removeSecret(secret); err != nil {
			removalErrors = append(removalErrors, err)
		}
	}

	return utilerrors.NewAggregate(removalErrors)
}

// HasSecret returns if a secret-refresh has been scheduled in cronAgent or not
func (c *Cron) HasSecret(name string) bool {
	c.lock.Lock()
	defer c.lock.Unlock()

	_, ok := c.secrets[name]
	return ok
}

func (c *Cron) addPeriodic(spec RotatedSecretSpec) error {
	if secret, ok := c.secrets[spec.String()]; ok {
		if secret.cronStr == spec.Refresh.Cron {
			return nil
		}
		// cron updated, remove old entry
		if err := c.removeSecret(spec.String()); err != nil {
			return err
		}
	}

	// refreshStrategy set to refreshInterval, skip this periodic
	if spec.Refresh.Cron == "" {
		return nil
	}

	if err := c.addSecret(spec.String(), spec.Refresh.Cron); err != nil {
		return err
	}

	return nil
}

// addSecret adds a cron entry for a secret-refresh to cronAgent
func (c *Cron) addSecret(name, cron string) error {
	id, err := c.cronAgent.AddFunc("TZ=UTC "+cron, func() {
		c.lock.Lock()
		defer c.lock.Unlock()

		c.secrets[name].triggered = true
	})

	if err != nil {
		return fmt.Errorf("cronAgent fails to add refresh for %s with cron %s: %v", name, cron, err)
	}

	c.secrets[name] = &secretStatus{
		entryID:   id,
		cronStr:   cron,
		triggered: false,
	}

	return nil
}

// removeSecret removes the secret-refresh from cronAgent
func (c *Cron) removeSecret(name string) error {
	secret, ok := c.secrets[name]
	if !ok {
		return fmt.Errorf("job %s has not been added to cronAgent yet", name)
	}
	c.cronAgent.Remove(secret.entryID)
	delete(c.secrets, name)
	return nil
}
