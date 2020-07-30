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
	"strconv"
	"time"

	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

// MockClient mocks a Secret Manager client
// MockClient.Secrets is a map from <project>, <secret_id> to a Secret object
type MockClient struct {
	Secrets map[string]map[string]*Secret
	// Note that a new version added through MockClient.UpsertSecret() will just have
	// a zero value of CreateTime. CreateTime generation is not mocked.
}

// Secret mocks a Secret Manager secret, which contains metadata and a list of versions
// Secret.Versions is a map from <version> to a secret Version object
// Secret.Labels stores the secret metadata
type Secret struct {
	Versions map[string]*Version
	Labels   map[string]string
}

// Version mocks a Secret Manager secret version, which contains version CreateTime, secret Data and version State
type Version struct {
	CreateTime time.Time
	Data       []byte
	State      secretmanagerpb.SecretVersion_State
}

// ValidateProject returns nil if the project exists, otherwise error.
func (cl *MockClient) ValidateProject(project string) error {
	_, ok := cl.Secrets[project]
	if !ok {
		return status.Error(codes.NotFound, fmt.Sprintf("Project [projects/%s] not found.", project))
	}

	return nil
}

// ValidateSecret returns nil if the secret exists, otherwise error.
func (cl *MockClient) ValidateSecret(project, id string) error {
	_, ok := cl.Secrets[project][id]
	if !ok {
		return status.Error(codes.NotFound, fmt.Sprintf("Secret [projects/%s/secrets/%s] not found or has no versions.", project, id))
	}

	return nil
}

// ValidateSecretVersion returns nil if the secret version exists, otherwise error.
func (cl *MockClient) ValidateSecretVersion(project, id, version string) error {
	err := cl.ValidateSecret(project, id)
	if err != nil {
		return err
	}

	if version == "latest" {
		version = strconv.Itoa(len(cl.Secrets[project][id].Versions))
	}

	_, ok := cl.Secrets[project][id].Versions[version]
	if !ok {
		return status.Error(codes.NotFound, fmt.Sprintf("Secret Version [projects/%s/secrets/%s/versions/%s] not found.", project, id, version))
	}

	return nil
}

// UpsertSecret adds a new version to the secret specified by project, id.
// It inserts a new secret if id doesn't already exist.
// If successful the latest version will have 'data' as its secret value,
// and return the latest version number if successful, otherwise returns error
func (cl *MockClient) UpsertSecret(project, id string, data []byte) (string, error) {
	err := cl.ValidateProject(project)
	if err != nil {
		return "", err
	}

	err = cl.ValidateSecret(project, id)
	if err != nil {
		cl.Secrets[project][id] = new(Secret)
	}

	version := strconv.Itoa(len(cl.Secrets[project][id].Versions) + 1)
	cl.Secrets[project][id].Versions[version] = &Version{
		Data:  data,
		State: secretmanagerpb.SecretVersion_ENABLED,
	}

	return version, nil
}

// GetCreateTime gets the createTime of the secret version specified by project, id, version.
// Returns createTime if successful, otherwise error.
func (cl *MockClient) GetCreateTime(project, id, version string) (time.Time, error) {
	err := cl.ValidateSecretVersion(project, id, version)
	if err != nil {
		return time.Time{}, err
	}

	if version == "latest" {
		version = strconv.Itoa(len(cl.Secrets[project][id].Versions))
	}

	creatTime := cl.Secrets[project][id].Versions[version].CreateTime

	return creatTime, nil
}

// GetSecretLabels gets the labels of the secret specified by project, id.
// Returns secret labels if successful, otherwise error
func (cl *MockClient) GetSecretLabels(project, id string) (map[string]string, error) {
	err := cl.ValidateSecret(project, id)
	if err != nil {
		return nil, err
	}

	ret := make(map[string]string)
	for key, val := range cl.Secrets[project][id].Labels {
		ret[key] = val
	}

	return ret, nil
}

// GetSecretVersionData gets the data of the secret version specified by project, id, version.
// Returns secret value if successful, otherwise error
func (cl *MockClient) GetSecretVersionData(project, id, version string) ([]byte, error) {
	err := cl.ValidateSecretVersion(project, id, version)
	if err != nil {
		return nil, err
	}

	if version == "latest" {
		version = strconv.Itoa(len(cl.Secrets[project][id].Versions))
	}

	return cl.Secrets[project][id].Versions[version].Data, nil
}

// GetSecretVersionState gets the state of the secret version specified by project, id, version.
// Returns state if successful, otherwise error.
func (cl *MockClient) GetSecretVersionState(project, id, version string) (secretmanagerpb.SecretVersion_State, error) {
	err := cl.ValidateSecretVersion(project, id, version)
	if err != nil {
		return 0, err
	}

	if version == "latest" {
		version = strconv.Itoa(len(cl.Secrets[project][id].Versions))
	}

	return cl.Secrets[project][id].Versions[version].State, nil
}

// EnableSecretVersion changes the state of secret version to ENABLED
// returns nil if successful, otherwise error.
func (cl *MockClient) EnableSecretVersion(project, id, version string) error {
	err := cl.ValidateSecretVersion(project, id, version)
	if err != nil {
		return err
	}

	if version == "latest" {
		version = strconv.Itoa(len(cl.Secrets[project][id].Versions))
	}

	cl.Secrets[project][id].Versions[version].State = secretmanagerpb.SecretVersion_ENABLED

	return nil
}

// DisableSecretVersion changes the state of secret version to DISABLED
// returns nil if successful, otherwise error.
func (cl *MockClient) DisableSecretVersion(project, id, version string) error {
	err := cl.ValidateSecretVersion(project, id, version)
	if err != nil {
		return err
	}

	if version == "latest" {
		version = strconv.Itoa(len(cl.Secrets[project][id].Versions))
	}

	cl.Secrets[project][id].Versions[version].State = secretmanagerpb.SecretVersion_DISABLED

	return nil
}

// DestroySecretVersion changes the state of secret version to DESTROYED
// returns nil if successful, otherwise error.
func (cl *MockClient) DestroySecretVersion(project, id, version string) error {
	err := cl.ValidateSecretVersion(project, id, version)
	if err != nil {
		return err
	}

	if version == "latest" {
		version = strconv.Itoa(len(cl.Secrets[project][id].Versions))
	}

	cl.Secrets[project][id].Versions[version].State = secretmanagerpb.SecretVersion_DESTROYED

	return nil
}

// UpsertSecretLabel updates or inserts the key-value pair
// in labels of the secret specified by project, id, key.
// Returns error if update fails or the secret doesn't exist.
func (cl *MockClient) UpsertSecretLabel(project, id, key, val string) error {
	err := cl.ValidateSecret(project, id)
	if err != nil {
		return err
	}

	cl.Secrets[project][id].Labels[key] = val

	return nil
}

// DeleteSecretLabel deletes the key-value pair
// in labels of the secret specified by project, id, key.
// Returns error if update fails or the secret doesn't exist.
func (cl *MockClient) DeleteSecretLabel(project, id, key string) error {
	err := cl.ValidateSecret(project, id)
	if err != nil {
		return err
	}

	delete(cl.Secrets[project][id].Labels, key)

	return nil
}
