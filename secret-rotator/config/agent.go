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

// This is a config agent for RotatedSecretConfig.
// It watches the mounted configMap, and updates the RotatedSecretConfig accordingly.

import (
	"context"
	"fmt"
	"k8s.io/apimachinery/pkg/util/sets"
	"k8s.io/klog"
	prow "k8s.io/test-infra/prow/config"
	"path/filepath"
	"sync"
)

type Agent struct {
	mutex  sync.RWMutex
	config *RotatedSecretConfig
	cron   *Cron
}

func NewAgent() *Agent {
	agent := &Agent{
		config: &RotatedSecretConfig{},
		cron:   NewCron(),
	}
	agent.cron.Start()
	return agent
}

// WatchConfig will begin watching the config file at the provided configPath.
// If the first load or valiadate fails, WatchConfig will return the error and abort.
// Future load or valiadate failures will be logged but continue to attempt loading config.
func (a *Agent) WatchConfig(configPath string) (func(ctx context.Context), error) {
	updateFunc := func() error {
		newConfig := &RotatedSecretConfig{}
		err := newConfig.LoadFrom(configPath)
		if err != nil {
			return fmt.Errorf("Fail to load config: %s", err)
		}

		err = newConfig.Validate()
		if err != nil {
			return fmt.Errorf("Fail to validate config: %s", err)
		}

		a.Set(newConfig)
		err = a.cron.SyncConfig(a.Config())
		return err
	}

	errFunc := func(err error, msg string) {
		klog.Errorf("Fail to get ConfigMap watcher: %s: %s", err, msg)
	}

	err := updateFunc()
	if err != nil {
		return nil, err
	}

	runFunc, err := prow.GetCMMountWatcher(updateFunc, errFunc, filepath.Dir(configPath))

	return runFunc, err
}

func (a *Agent) Config() *RotatedSecretConfig {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.config
}

func (a *Agent) CronQueuedSecrets() sets.String {
	a.mutex.RLock()
	defer a.mutex.RUnlock()
	return a.cron.QueuedSecrets()
}

func (a *Agent) Set(newConfig *RotatedSecretConfig) {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	a.config = newConfig
}
