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

package client

// Package client implements creation and utilities
// for K8s clientset and Secret Manager client.

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"os"
	"path/filepath"

	"k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	types "k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	_ "k8s.io/client-go/plugin/pkg/client/auth/gcp"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

// NewK8sClientset creates a new K8s clientset
// uses in-cluster configuration if possible, but falls back to out-of-cluster configuration otherwise.
// It loads from kubeconfig, and looks for a config file under $HOME if kubeconfig is not specified.
func NewK8sClientset(kubeconfig string) (*kubernetes.Interface, error) {
	// tries to create the in-cluster config
	config, err := rest.InClusterConfig()
	if err != nil {
		// fall back to using out-of-cluster config
		if kubeconfig == "" {
			// find the kubeconfig file under $HOME if no kubeconfig flag passed
			kubeconfig = filepath.Join(os.Getenv("HOME"), ".kube", "config")
		}
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
		if err != nil {
			return nil, err
		}
	}

	var clientset kubernetes.Interface
	clientset, err = kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}

	return &clientset, nil
}

func NewSecretManagerClient(ctx context.Context) (*secretmanager.Client, error) {
	client, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return client, nil
}

// structs for client interface
type Interface interface {
	ValidateKubernetesNamespace(namespace string) error
	ValidateKubernetesSecret(namespace, id string) error
	CreateKubernetesNamespace(namespace string) error
	GetKubernetesSecretValue(namespace, id, key string) ([]byte, error)
	UpsertKubernetesSecret(namespace, id, key string, data []byte) error
	GetSecretManagerSecretValue(project, id string) ([]byte, error)
	UpsertSecretManagerSecret(project, id string, data []byte) error
}
type Client struct { // actual client
	K8sClientset        kubernetes.Interface
	SecretManagerClient secretmanager.Client
}

// ValidateKubernetesNamespace returns nil if the namespace exists, otherwise error.
func (cl *Client) ValidateKubernetesNamespace(namespace string) error {
	_, err := cl.K8sClientset.CoreV1().Namespaces().Get(namespace, metav1.GetOptions{})
	return err
}

// ValidateKubernetesSecret returns nil if the secret exists under namespace, otherwise error.
func (cl *Client) ValidateKubernetesSecret(namespace, id string) error {
	_, err := cl.K8sClientset.CoreV1().Secrets(namespace).Get(id, metav1.GetOptions{})
	return err
}

// CreateKubernetesNamespace creates a K8s namesapce.
// Returns nil if successful, error otherwise
func (cl *Client) CreateKubernetesNamespace(namespace string) error {
	newNamespace := &v1.Namespace{
		ObjectMeta: metav1.ObjectMeta{
			Name: namespace,
		},
	}
	_, err := cl.K8sClientset.CoreV1().Namespaces().Create(newNamespace)
	return err
}

// GetKubernetesSecretValue gets the value of key from the kubernetes secret specified by namespace, id.
// Returns error if the namspace doesn't exist, otherwise nil if the secret or key don't exist.
func (cl *Client) GetKubernetesSecretValue(namespace, id, key string) ([]byte, error) {
	// check if namespace exists
	err := cl.ValidateKubernetesNamespace(namespace)
	if err != nil {
		return nil, err
	}

	secret, err := cl.K8sClientset.CoreV1().Secrets(namespace).Get(id, metav1.GetOptions{})
	if err != nil {
		if apierrors.IsNotFound(err) {
			return nil, nil
		} else {
			return nil, err
		}
	}

	// Get the secret value according to the key.
	value, ok := secret.Data[key]
	if !ok {
		return nil, nil
	}

	return value, nil
}

// UpsertKubernetesSecret updates the value of key of the kubernetes secret specified by namespace, id.
// It inserts a new secret if id doesn't already exist.
// It inserts a new key-value pair if key doesn't already exist.
// Returns nil if successful, error otherwise
func (cl *Client) UpsertKubernetesSecret(namespace, id, key string, data []byte) error {
	// check if the namespace exists
	err := cl.ValidateKubernetesNamespace(namespace)
	if err != nil {
		return err
	}

	// encode with base64 encoding
	encodedSrc := base64.StdEncoding.EncodeToString(data)
	patch, err := json.Marshal(map[string]interface{}{
		"data": map[string]string{key: encodedSrc},
	})
	if err != nil {
		return err
	}
	_, err = cl.K8sClientset.CoreV1().Secrets(namespace).Patch(id, types.StrategicMergePatchType, []byte(patch))
	if err != nil {
		if !apierrors.IsNotFound(err) {
			return err
		}

		// create a new secret in the case that it does not already exist
		newSecret := &v1.Secret{
			ObjectMeta: metav1.ObjectMeta{
				Name:      id,
				Namespace: namespace,
			},
			Data: map[string][]byte{
				key: data,
			},
		}
		_, err = cl.K8sClientset.CoreV1().Secrets(namespace).Create(newSecret)
		if err != nil {
			return err
		}
	}

	return nil
}

// UpsertSecretManagerSecret adds a new version to the Secret Manager secret specified by project, id.
// It inserts a new secret if id doesn't already exist.
// If successful the latest version will have 'data' as its secret value, otherwise return error
func (cl *Client) UpsertSecretManagerSecret(project, id string, data []byte) error {
	parent := "projects/" + project
	// Check if the secret exists
	_, err := cl.GetSecretManagerSecretValue(project, id)
	if err != nil {
		if status.Code(err) == codes.NotFound {
			// Create secret
			req := &secretmanagerpb.CreateSecretRequest{
				Parent:   parent,
				SecretId: id,
				Secret: &secretmanagerpb.Secret{
					Replication: &secretmanagerpb.Replication{
						Replication: &secretmanagerpb.Replication_Automatic_{
							Automatic: &secretmanagerpb.Replication_Automatic{},
						},
					},
				},
			}
			_, err := cl.SecretManagerClient.CreateSecret(context.TODO(), req)
			if err != nil {
				return err
			}
		} else {
			return err
		}
	}

	// Add secret version
	verReq := &secretmanagerpb.AddSecretVersionRequest{
		Parent: parent + "/secrets/" + id,
		Payload: &secretmanagerpb.SecretPayload{
			Data: data,
		},
	}
	_, err = cl.SecretManagerClient.AddSecretVersion(context.TODO(), verReq)
	if err != nil {
		return err
	}
	return nil
}

// GetSecretManagerSecretValue gets the value from the Secret Manager secret specified by project, id.
// Returns nil and secret value if successful, error otherwise
func (cl *Client) GetSecretManagerSecretValue(project, id string) ([]byte, error) {
	ctx := context.TODO()
	name := "projects/" + project + "/secrets/" + id + "/versions/latest"

	accReq := &secretmanagerpb.AccessSecretVersionRequest{
		Name: name,
	}
	accResult, err := cl.SecretManagerClient.AccessSecretVersion(ctx, accReq)
	if err != nil {
		return nil, err
	}

	return accResult.Payload.Data, nil
}
