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

// Package client implements Secret Manager client creation and utilities.

import (
	"context"
	"github.com/golang/protobuf/ptypes"
	"google.golang.org/genproto/protobuf/field_mask"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

type Client struct {
	*secretmanager.Client
}

func NewClient(ctx context.Context) (*Client, error) {
	gsmClient, err := secretmanager.NewClient(ctx)
	if err != nil {
		return nil, err
	}
	return &Client{gsmClient}, nil
}

type Interface interface {
	ValidateSecret(project, id string) error
	ValidateSecretVersion(project, id, version string) error
	UpsertSecret(project, id string, data []byte) (string, error)
	GetCreateTime(project, id, version string) (time.Time, error)
	GetSecretLabels(project, id string) (map[string]string, error)
	UpsertSecretLabel(project, id, key, val string) error
	DeleteSecretLabel(project, id, key string) error
}

// ValidateSecret returns nil if the secret exists, otherwise error.
func (cl *Client) ValidateSecret(project, id string) error {
	ctx := context.TODO()
	name := "projects/" + project + "/secrets/" + id

	getReq := &secretmanagerpb.GetSecretRequest{
		Name: name,
	}
	_, err := cl.GetSecret(ctx, getReq)

	return err
}

// ValidateSecretVersion returns nil if the secret version exists, otherwise error.
func (cl *Client) ValidateSecretVersion(project, id, version string) error {
	ctx := context.TODO()
	name := "projects/" + project + "/secrets/" + id + "/versions/" + version

	getReq := &secretmanagerpb.GetSecretVersionRequest{
		Name: name,
	}
	_, err := cl.GetSecretVersion(ctx, getReq)

	return err
}

// UpsertSecret adds a new version to the secret specified by project, id.
// It inserts a new secret if id doesn't already exist.
// If successful the latest version will have 'data' as its secret value, otherwise return error
func (cl *Client) UpsertSecret(project, id string, data []byte) (string, error) {
	parent := "projects/" + project
	// Check if the secret exists
	err := cl.ValidateSecret(project, id)
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
			_, err := cl.CreateSecret(context.TODO(), req)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}

	// Add secret version
	verReq := &secretmanagerpb.AddSecretVersionRequest{
		Parent: parent + "/secrets/" + id,
		Payload: &secretmanagerpb.SecretPayload{
			Data: data,
		},
	}
	verResp, err := cl.AddSecretVersion(context.TODO(), verReq)
	if err != nil {
		return "", err
	}
	return verResp.Name, nil
}

// GetCreateTime gets the createTime of the secret version specified by project, id, version.
// Returns error if the secret version doesn't exist.
func (cl *Client) GetCreateTime(project, id, version string) (time.Time, error) {
	ctx := context.TODO()
	name := "projects/" + project + "/secrets/" + id + "/versions/" + version

	getReq := &secretmanagerpb.GetSecretVersionRequest{
		Name: name,
	}
	getResult, err := cl.GetSecretVersion(ctx, getReq)
	if err != nil {
		return time.Time{}, err
	}

	createTime, err := ptypes.Timestamp(getResult.CreateTime)
	if err != nil {
		return time.Time{}, err
	}

	return createTime, nil
}

// GetSecretLabels gets the labels of the secret specified by project, id.
// Returns error if the secret doesn't exist.
func (cl *Client) GetSecretLabels(project, id string) (map[string]string, error) {
	ctx := context.TODO()
	name := "projects/" + project + "/secrets/" + id

	getReq := &secretmanagerpb.GetSecretRequest{
		Name: name,
	}
	getResult, err := cl.GetSecret(ctx, getReq)
	if err != nil {
		return nil, err
	}

	return getResult.Labels, nil
}

// UpsertSecretLabel updates or inserts the key-value pair
// in labels of the secret specified by project, id, key.
// Returns error if update fails or the secret doesn't exist.
func (cl *Client) UpsertSecretLabel(project, id, key, val string) error {
	ctx := context.TODO()
	name := "projects/" + project + "/secrets/" + id

	labels, err := cl.GetSecretLabels(project, id)
	if err != nil {
		return err
	}

	// update or insert new label
	labels[key] = val

	updateReq := &secretmanagerpb.UpdateSecretRequest{
		Secret: &secretmanagerpb.Secret{
			Name:   name,
			Labels: labels,
		},
		UpdateMask: &field_mask.FieldMask{
			Paths: []string{"labels"},
		},
	}
	_, err = cl.UpdateSecret(ctx, updateReq)

	return err
}

// DeleteSecretLabel deletes the key-value pair
// in labels of the secret specified by project, id, key.
// Returns error if update fails or the secret doesn't exist.
func (cl *Client) DeleteSecretLabel(project, id, key string) error {
	ctx := context.TODO()
	name := "projects/" + project + "/secrets/" + id

	labels, err := cl.GetSecretLabels(project, id)
	if err != nil {
		return err
	}

	delete(labels, key)

	updateReq := &secretmanagerpb.UpdateSecretRequest{
		Secret: &secretmanagerpb.Secret{
			Name:   name,
			Labels: labels,
		},
		UpdateMask: &field_mask.FieldMask{
			Paths: []string{"labels"},
		},
	}
	_, err = cl.UpdateSecret(ctx, updateReq)

	return err
}
