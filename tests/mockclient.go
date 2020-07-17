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
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

type MockClient struct { // mock client
	K8sSecret           map[string]map[string]map[string][]byte
	SecretManagerSecret map[string]map[string][]byte
}

func NewMockClient(namespaces []string) *MockClient {
	mock := MockClient{
		K8sSecret:           make(map[string]map[string]map[string][]byte),
		SecretManagerSecret: make(map[string]map[string][]byte),
	}

	for _, ns := range namespaces {
		mock.SecretManagerSecret[ns] = make(map[string][]byte)
	}
	return &mock
}

func (cl *MockClient) ValidateKubernetesNamespace(namespace string) error {
	_, ok := cl.K8sSecret[namespace]
	if !ok {
		return apierrors.NewNotFound(schema.GroupResource{"", "namespaces"}, namespace)
	}
	return nil
}
func (cl *MockClient) ValidateKubernetesSecret(namespace, id string) error {
	_, ok := cl.K8sSecret[namespace][id]
	if !ok {
		return apierrors.NewNotFound(schema.GroupResource{"", "secrets"}, id)
	}
	return nil
}
func (cl *MockClient) CreateKubernetesNamespace(namespace string) error {
	cl.K8sSecret[namespace] = make(map[string]map[string][]byte)
	return nil
}
func (cl *MockClient) GetKubernetesSecretValue(namespace, id, key string) ([]byte, error) {
	err := cl.ValidateKubernetesNamespace(namespace)
	if err != nil {
		return nil, err
	}

	err = cl.ValidateKubernetesSecret(namespace, id)
	if err != nil {
		return nil, nil
	}
	val, ok := cl.K8sSecret[namespace][id][key]
	if !ok {
		return nil, nil
	}
	return val, nil
}
func (cl *MockClient) UpsertKubernetesSecret(namespace, id, key string, data []byte) error {
	err := cl.ValidateKubernetesNamespace(namespace)
	if err != nil {
		return err
	}

	err = cl.ValidateKubernetesSecret(namespace, id)
	if err != nil {
		cl.K8sSecret[namespace][id] = make(map[string][]byte)
	}
	cl.K8sSecret[namespace][id][key] = data

	return nil
}
func (cl *MockClient) UpsertKubernetesConfigMap(namespace, name, key, data string) error {
	return nil
}
func (cl *MockClient) CreateKubernetesSecret(namespace, id string) error {
	err := cl.ValidateKubernetesNamespace(namespace)
	if err != nil {
		return err
	}

	err = cl.ValidateKubernetesSecret(namespace, id)
	if err == nil {
		return fmt.Errorf("secret \"%s\" already exists", id)
	}
	cl.K8sSecret[namespace][id] = make(map[string][]byte)

	return nil
}
func (cl *MockClient) GetSecretManagerSecretValue(project, id string) ([]byte, error) {
	val, ok := cl.SecretManagerSecret[project][id]
	if !ok {
		return nil, status.Error(codes.NotFound, fmt.Sprintf("Secret [projects/%s/secrets/%s] not found or has no versions.", project, id))
	}
	return val, nil
}
func (cl *MockClient) UpsertSecretManagerSecret(project, id string, data []byte) error {
	_, ok := cl.SecretManagerSecret[project]
	if !ok {
		return status.Error(codes.NotFound, fmt.Sprintf("Secret [projects/%s] not found.", project))
	}
	cl.SecretManagerSecret[project][id] = data
	return nil
}
func (cl *MockClient) DeleteSecretManagerSecret(project, id string) error {
	delete(cl.SecretManagerSecret[project], id)
	return nil
}
func (cl *MockClient) CleanupKubernetesNamespace(namespace string) error {
	delete(cl.K8sSecret, namespace)
	return nil
}
func (cl *MockClient) CleanupKubernetesSecrets(namespace string) error {
	cl.K8sSecret[namespace] = make(map[string]map[string][]byte)
	return nil
}
