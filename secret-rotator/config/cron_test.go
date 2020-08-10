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
	"testing"
	"time"

	cron "gopkg.in/robfig/cron.v2"
)

var str2Duration = func(str string) time.Duration {
	d, _ := time.ParseDuration(str)
	return d
}

func TestSyncConfig(t *testing.T) {

	initConfig := &RotatedSecretConfig{
		Specs: []RotatedSecretSpec{
			{
				Project: "project-1",
				Secret:  "secret-1",
				Refresh: RefreshStrategy{
					Interval: str2Duration("48h"),
				},
			},
			{
				Project: "project-2",
				Secret:  "secret-2",
				Refresh: RefreshStrategy{
					Cron: "0 0 * * 1",
				},
			},
			{
				Project: "project-3",
				Secret:  "secret-3",
				Refresh: RefreshStrategy{
					Cron: "0 0 * * 1",
				},
			},
			{
				Project: "project-4",
				Secret:  "secret-4",
				Refresh: RefreshStrategy{
					Cron: "0 0 * * 1",
				},
			},
		},
	}

	shouldHaveInit := map[string]bool{
		"SecretManager:/projects/project-1/secrets/secret-1": false,
		"SecretManager:/projects/project-2/secrets/secret-2": true,
		"SecretManager:/projects/project-3/secrets/secret-3": true,
		"SecretManager:/projects/project-4/secrets/secret-4": true,
	}

	newConfig := &RotatedSecretConfig{
		Specs: []RotatedSecretSpec{
			{
				// secret-1 should be added into cron.secrets
				Project: "project-1",
				Secret:  "secret-1",
				Refresh: RefreshStrategy{
					Cron: "0 8 * * 1",
				},
			},
			{
				// secret-2 should be removed from cron.secrets
				Project: "project-2",
				Secret:  "secret-2",
				Refresh: RefreshStrategy{
					Interval: str2Duration("48h"),
				},
			},
			{
				// secret-3 should stay in cron.secrets
				Project: "project-3",
				Secret:  "secret-3",
				Refresh: RefreshStrategy{
					Cron: "0 0 * * 1",
				},
			},
			{
				// secret-4 should be in cron.secrets, having a new cronID
				Project: "project-4",
				Secret:  "secret-4",
				Refresh: RefreshStrategy{
					Cron: "0 8 * * 1",
				},
			},
		},
	}

	shouldHaveAfter := map[string]bool{
		"SecretManager:/projects/project-1/secrets/secret-1": true,
		"SecretManager:/projects/project-2/secrets/secret-2": false,
		"SecretManager:/projects/project-3/secrets/secret-3": true,
		"SecretManager:/projects/project-4/secrets/secret-4": true,
	}

	shouldUpdateAfter := map[string]bool{
		"SecretManager:/projects/project-3/secrets/secret-3": false,
		"SecretManager:/projects/project-4/secrets/secret-4": true,
	}

	c := NewCron()
	cronIDs := map[string]cron.EntryID{}

	// initial config
	if err := c.SyncConfig(initConfig); err != nil {
		t.Fatalf("error first sync config: %v", err)
	}

	for secretName, shouldHave := range shouldHaveInit {
		if !shouldHave && c.HasSecret(secretName) {
			t.Errorf("Initial sync, should not have secret '%s' in cron", secretName)
		} else if shouldHave && !c.HasSecret(secretName) {
			t.Errorf("Initial sync, should have secret '%s' in cron", secretName)
		} else if shouldHave && c.HasSecret(secretName) {
			cronIDs[secretName] = c.secrets[secretName].entryID
		}
	}

	// new config
	if err := c.SyncConfig(newConfig); err != nil {
		t.Fatalf("error sync new config: %v", err)
	}

	for secretName, shouldHave := range shouldHaveAfter {
		if !shouldHave && c.HasSecret(secretName) {
			t.Errorf("Second sync, should not have secret '%s' in cron", secretName)
		} else if shouldHave && !c.HasSecret(secretName) {
			t.Errorf("Second sync, should have secret '%s' in cron", secretName)
		}
	}

	for secretName, shouldUpdate := range shouldUpdateAfter {
		if shouldUpdate && cronIDs[secretName] == c.secrets[secretName].entryID {
			t.Errorf("Second sync, cron entryID for secret '%s' should have been updated", secretName)
		} else if !shouldUpdate && cronIDs[secretName] != c.secrets[secretName].entryID {
			t.Errorf("Second sync, cron entryID for secret '%s' should not have been updated", secretName)
		}
	}
}

func TestTrigger(t *testing.T) {
	cfg := &RotatedSecretConfig{
		Specs: []RotatedSecretSpec{
			{
				Project: "project-1",
				Secret:  "secret-1",
				Refresh: RefreshStrategy{
					Interval: str2Duration("48h"),
				},
			},
			{
				Project: "project-2",
				Secret:  "secret-2",
				Refresh: RefreshStrategy{
					Cron: "0 0 * * 1",
				},
			},
			{
				Project: "project-3",
				Secret:  "secret-3",
				Refresh: RefreshStrategy{
					Cron: "0 0 * * 1",
				},
			},
			{
				Project: "project-4",
				Secret:  "secret-4",
				Refresh: RefreshStrategy{
					Cron: "0 0 * * 1",
				},
			},
		},
	}

	shouldBeTriggered := map[string]bool{
		"SecretManager:/projects/project-1/secrets/secret-1": false,
		"SecretManager:/projects/project-2/secrets/secret-2": true,
		"SecretManager:/projects/project-3/secrets/secret-3": true,
		"SecretManager:/projects/project-4/secrets/secret-4": true,
	}

	c := NewCron()

	if err := c.SyncConfig(cfg); err != nil {
		t.Fatalf("error sync config: %v", err)
	}

	for secret := range c.QueuedSecrets() {
		t.Errorf("intial sync, should not have triggered secret '%s'", secret)
	}

	// force trigger
	for _, entry := range c.cronAgent.Entries() {
		entry.Job.Run()
	}

	triggered := c.QueuedSecrets()

	for secret, should := range shouldBeTriggered {
		if !should && triggered.Has(secret) {
			t.Errorf("should not have triggered secret '%s'", secret)
		} else if should && !triggered.Has(secret) {
			t.Errorf("should have triggered secret '%s'", secret)
		}
	}
}
