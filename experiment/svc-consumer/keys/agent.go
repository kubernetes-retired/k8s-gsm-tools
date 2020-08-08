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

package keys

// This is an agent for rotated service account keys.
// It watches the mounted secret (service account key), and appends to its keys slice when an update occurs.

import (
	"context"
	"fmt"
	"io"
	"k8s.io/klog"
	prow "k8s.io/test-infra/prow/config"
	"os"
	"path/filepath"
	"strconv"
	"sync"
)

type Agent struct {
	mutex sync.RWMutex
	keys  []string
	Dir   string
}

// WatchMounted will begin watching the secret file at the provided mountPath.
// If the first load fails, WatchMounted will return the error and abort.
// Future failures will be logged but continue to attempt loading and adding key.
func (a *Agent) WatchMounted(mountPath string) (func(ctx context.Context), error) {
	updateFunc := func() error {
		err := a.AddNewKey(mountPath)
		if err != nil {
			return err
		}

		return nil
	}

	errFunc := func(err error, msg string) {
		klog.Errorf("Fail to get watcher: %s: %s", err, msg)
	}

	err := updateFunc()
	if err != nil {
		return nil, err
	}

	runFunc, err := prow.GetCMMountWatcher(updateFunc, errFunc, filepath.Dir(mountPath))
	if err != nil {
		klog.Error(err)
	}
	return runFunc, err
}

// GetKeys gets the slice of all key filenames in Agent.keys
func (a *Agent) GetKeys() []string {
	a.mutex.RLock()
	defer a.mutex.RUnlock()

	if len(a.keys) == 0 {
		return nil
	}

	return a.keys
}

// AddNewKey copies the current keyfile in mountPath into Agent.Dir,
// where Agent.Dir is the desired directory for storing all versions of keyfile that ever existed.
// renames the copied keyfile according to the version number and appends the new filename into Agent.keys
func (a *Agent) AddNewKey(mountPath string) error {
	a.mutex.Lock()
	defer a.mutex.Unlock()

	stat, err := os.Stat(mountPath)
	if err != nil {
		return err
	}

	if !stat.Mode().IsRegular() {
		return fmt.Errorf("%s is not a regular file", mountPath)
	}

	source, err := os.Open(mountPath)
	if err != nil {
		return err
	}

	defer source.Close()

	os.Mkdir(a.Dir, 0755)

	copy := filepath.Join(a.Dir, "key_"+strconv.Itoa(len(a.keys)+1))
	destination, err := os.Create(copy)
	if err != nil {
		return err
	}

	defer destination.Close()

	_, err = io.Copy(destination, source)
	if err != nil {
		return err
	}

	a.keys = append(a.keys, copy)

	return nil
}
