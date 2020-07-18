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

// Package config defines configuration and sync-pair structs

import (
	"context"
	"fmt"
	"k8s.io/klog"
	prow "k8s.io/test-infra/prow/config"
	"path/filepath"
	"sync"
)

type Agent struct {
	mutex  sync.RWMutex
	config *SecretSyncConfig
}

// WatchConfig will begin watching the config file at the provided configPath.
// If the first load or valiadate fails, WatchConfig will return the error and abort.
// Future load or valiadate failures will be logged but continue to attempt loading config.
func (ca *Agent) WatchConfig(configPath string) (func(ctx context.Context), error) {
	updateFunc := func() error {
		newConfig := &SecretSyncConfig{}
		err := newConfig.LoadFrom(configPath)
		if err != nil {
			return fmt.Errorf("Fail to load config: %s", err)
		}

		err = newConfig.Validate()
		if err != nil {
			return fmt.Errorf("Fail to validate config: %s", err)
		}

		ca.Set(newConfig)
		return nil
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

func (ca *Agent) Config() *SecretSyncConfig {
	ca.mutex.RLock()
	defer ca.mutex.RUnlock()
	return ca.config
}

func (ca *Agent) Set(newConfig *SecretSyncConfig) {
	ca.mutex.Lock()
	defer ca.mutex.Unlock()

	ca.config = newConfig
}
