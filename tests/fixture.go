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
	"bytes"
	"context"
	"gopkg.in/yaml.v2"
	"strings"
	"text/template"
	"time"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"

	"github.com/b01901143/secret-sync-controller/pkg/client"
)

type ClientInterface interface {
	CreateKubernetesSecret(namespace, id string) error
	DeleteSecretManagerSecret(project, id string) error
	CleanupKubernetesNamespace(namespace string) error
	CleanupKubernetesSecrets(namespace string) error
	client.Interface
}

type E2eTestClient struct {
	*client.Client
}

// CreateKubernetesSecret creates an empty K8s secret under namespace.
// Returns nil if successful, error otherwise
func (cl *E2eTestClient) CreateKubernetesSecret(namespace, id string) error {
	newSecret := &v1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      id,
			Namespace: namespace,
		},
	}
	_, err := cl.K8sClientset.CoreV1().Secrets(namespace).Create(newSecret)
	return err
}

// DeleteSecretManagerSecret deletes the secret in the given project.
// Returns nil if succeeded, otherwise error.
func (cl *E2eTestClient) DeleteSecretManagerSecret(project, id string) error {
	name := "projects/" + project + "/secrets/" + id
	req := &secretmanagerpb.DeleteSecretRequest{
		Name: name,
	}
	if err := cl.SecretManagerClient.DeleteSecret(context.TODO(), req); err != nil {
		return err
	}
	return nil
}

// CleanupKubernetesNamespace deletes the given namespace in the cluster.
// Returns nil if succeeded, otherwise error.
func (cl *E2eTestClient) CleanupKubernetesNamespace(namespace string) error {
	// check if the namespace exists
	err := cl.ValidateKubernetesNamespace(namespace)
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		} else {
			return err
		}
	}
	err = cl.K8sClientset.CoreV1().Namespaces().Delete(namespace, &metav1.DeleteOptions{})
	if err != nil {
		return err
	}
	return nil
}

// CleanupKubernetesSecrets deletes all secrets expect for default-token-* under the given namespace in the cluster.
// Returns nil if succeeded, otherwise error.
func (cl *E2eTestClient) CleanupKubernetesSecrets(namespace string) error {
	// check if the namespace exists
	_, err := cl.K8sClientset.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil
		} else {
			return err
		}
	}
	secretList, err := cl.K8sClientset.CoreV1().Secrets(namespace).List(metav1.ListOptions{})

	if err != nil {
		return err
	}

	for _, secret := range secretList.Items {
		secretID := secret.ObjectMeta.Name
		if strings.HasPrefix(secretID, "default-token") {
			continue
		}
		// delete all secrets expect for default-token-*
		err := cl.K8sClientset.CoreV1().Secrets(namespace).Delete(secretID, &metav1.DeleteOptions{})
		if err != nil {
			return err
		}
	}
	return nil
}

type Fixture struct {
	// map of namespace to secret to key to value
	Kubernetes map[string]map[string]map[string]string
	// map of project to secret to value
	SecretManager map[string]map[string]string
}

func NewFixture(config string, project string) (f Fixture, err error) {
	t := template.Must(template.New("config").Parse(config))
	b := new(bytes.Buffer)
	t.Execute(b, project)

	err = yaml.Unmarshal(b.Bytes(), &f)

	return f, err
}

// Setup sets up the Secret Manager secrets and K8s namespaces with the Fixture and the given client.
// Returns nil if successful, error otherwise
func (f Fixture) Setup(cl ClientInterface) error {
	for project, projItem := range f.SecretManager {
		for secret, data := range projItem {
			err := cl.UpsertSecretManagerSecret(project, secret, []byte(data))
			if err != nil {
				return err
			}
		}
	}

	for namespace, _ := range f.Kubernetes {
		// check if the namespace exists
		err := cl.ValidateKubernetesNamespace(namespace)
		if err != nil {
			if apierrors.IsNotFound(err) {
				// this namespace does not exist yet
				err = cl.CreateKubernetesNamespace(namespace)
				if err != nil {
					return err
				}
			} else {
				return err
			}
		}
	}
	return nil
}

// Reset clears or re-creates the K8s secrets with the Fixture and the given client.
// Returns nil if successful, error otherwise
func (f Fixture) Reset(cl ClientInterface) error {
	for namespace, nsItem := range f.Kubernetes {
		if nsItem == nil {
			continue
		}
		err := cl.CleanupKubernetesSecrets(namespace)
		if err != nil {
			return err
		}

		for secret, secretItem := range nsItem {
			err = cl.CreateKubernetesSecret(namespace, secret)
			if err != nil {
				return err
			}

			if secretItem == nil {
				continue
			}
			for key, data := range secretItem {
				err = cl.UpsertKubernetesSecret(namespace, secret, key, []byte(data))
				if err != nil {
					return err
				}

			}
		}
	}
	return nil
}

// Teardown deletes all Secret Manager secrets and Kubernetes secrets&namespaces created by Setup().
// Returns nil if successful, error otherwise
func (f Fixture) Teardown(cl ClientInterface) error {
	for project, projItem := range f.SecretManager {
		for secret, _ := range projItem {
			err := cl.DeleteSecretManagerSecret(project, secret)
			if err != nil {
				return err
			}
		}
	}

	for namespace, _ := range f.Kubernetes {
		err := cl.CleanupKubernetesSecrets(namespace)
		if err != nil {
			return err
		}

		err = cl.CleanupKubernetesNamespace(namespace)
		if err != nil {
			return err
		}

		// wait until the namespace deletion completes
		for {
			err := cl.ValidateKubernetesNamespace(namespace)
			if err != nil {
				if !apierrors.IsNotFound(err) {
					return err
				} else {
					break
				}
			}
			time.Sleep(1000 * time.Millisecond)
		}
	}
	return nil
}
