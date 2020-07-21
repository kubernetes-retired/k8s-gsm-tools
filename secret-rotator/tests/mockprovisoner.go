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

package tests

// package client implements testing clients, mocked clients, and fixtures utilities.
// Should be used with caution. Only for testing purpose.

import (
	"fmt"
	"k8s.io/klog"
)

type MockSvcProvisioner struct {
	NewSecretID    string
	NewSecretValue []byte
}

// TODO: add project and service-account fields to metadata when registered

// CreateNew provisions a new service account key,
// returns the key-id and private-key data of the created key if successful,
// otherwise returns error
func (p *MockSvcProvisioner) CreateNew(labels map[string]string) (string, []byte, error) {
	// TODO: provision new service account key
	name := fmt.Sprintf("projects/%s/serviceAccounts/%s", labels["project"], labels["service-account"])
	klog.V(2).Infof("Provision a new secret of %s", name)
	return p.NewSecretID, p.NewSecretValue, nil
}

// Deactivate deletes an existing service account key specified by labels and version,
// returns nil if successful, otherwise error
func (p *MockSvcProvisioner) Deactivate(labels map[string]string, version string) error {
	// TODO: delete old service account key
	name := fmt.Sprintf("projects/%s/serviceAccounts/%s", labels["project"], labels["service-account"])
	klog.V(2).Infof("Deactivate ver. %s of %s", version, name)
	return nil
}
