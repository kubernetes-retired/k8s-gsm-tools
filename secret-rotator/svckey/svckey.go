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

package svckey

// package svckey implements the provisioning and deactivation of gcloud service account keys

import (
	"context"
	"encoding/base64"
	"fmt"
	"google.golang.org/api/iam/v1"
	"k8s.io/klog"
	"strings"
)

type ServiceAccountKeySpec struct {
	Project        string `yaml:"project"`
	ServiceAccount string `yaml:"serviceAccount"`
}

func (svc ServiceAccountKeySpec) String() string {
	return fmt.Sprintf("projects/%s/serviceAccounts/%s", svc.Project, svc.ServiceAccount)
}

// ServiceAccountKeySpec.Type() is used to obtain the provisioner of the ServiceAccountKey
func (svc ServiceAccountKeySpec) Type() string {
	return "serviceAccountKey"
}

// ServiceAccountKeySpec.Labels() is used to obtain the labels needed for the provisioner of the ServiceAccountKey
func (svc ServiceAccountKeySpec) Labels() map[string]string {
	return map[string]string{
		"project":         svc.Project,
		"service-account": svc.ServiceAccount,
	}
}

// Provisioner is a GCP service account key provisioner.
// It creates new svc-keys and deletes old svc-keys if enabled.
type Provisioner struct {
	// if enableDeletion is set to true, the provisioner deletes the old svc key of 'version'
	// when Deactivate('labels', 'version') is called.
	enableDeletion bool
	Service        *iam.Service
}

// NewProvisioner creates a new svc-key provisioner with a new iam service.
// The argument 'enableDeletion' specifies if deletion of old svc-keys is enabled.
// It returns a pointer to the new provisioner and any error if encountered.
func NewProvisioner(enableDeletion bool) (*Provisioner, error) {
	ctx := context.Background()

	service, err := iam.NewService(ctx)
	if err != nil {
		return nil, err
	}

	return &Provisioner{
		Service:        service,
		enableDeletion: enableDeletion,
	}, nil
}

// CreateNew provisions a new service account key,
// returns the key-id and private-key data of the created key if successful,
// otherwise returns error
func (p *Provisioner) CreateNew(labels map[string]string) (string, []byte, error) {
	name := fmt.Sprintf("projects/%s/serviceAccounts/%s@%s.iam.gserviceaccount.com", labels["project"], labels["service-account"], labels["project"])
	request := &iam.CreateServiceAccountKeyRequest{}

	resp, err := p.Service.Projects.ServiceAccounts.Keys.Create(name, request).Context(context.TODO()).Do()
	if err != nil {
		return "", nil, err
	}

	decodedPrivateKeyData, _ := base64.StdEncoding.DecodeString(resp.PrivateKeyData)

	splits := strings.Split(resp.Name, "/")
	key := splits[len(splits)-1]

	klog.V(2).Infof("Provisioned a new service account key %s/keys/%s", name, key)

	return key, decodedPrivateKeyData, nil
}

// Deactivate deletes an existing service account key specified by labels and version,
// returns nil if successful, otherwise error
func (p *Provisioner) Deactivate(labels map[string]string, version string) error {
	// keys in format of "v%d" indicate that they are (version: id) pairs attached by the rotator
	// the reason for the prefix "v" is that Secret Manager labels need to begin with a lowwer case letter
	name := fmt.Sprintf("projects/%s/serviceAccounts/%s@%s.iam.gserviceaccount.com/keys/%s", labels["project"], labels["service-account"], labels["project"], labels["v"+version])

	if p.enableDeletion {
		_, err := p.Service.Projects.ServiceAccounts.Keys.Delete(name).Do()
		if err != nil {
			return fmt.Errorf("Projects.ServiceAccounts.Keys.Delete: %v", err)
		}

		klog.V(2).Infof("Deactivated ver. %s: %s ", version, name)
	} else {
		klog.V(2).Infof("Should deactivate ver. %s: %s ", version, name)
	}

	return nil
}
