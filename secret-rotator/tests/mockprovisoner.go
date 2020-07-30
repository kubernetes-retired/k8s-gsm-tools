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
	"math/rand"
)

type MockSvcProvisioner struct {
	NewSecretID    string
	NewSecretValue []byte
}

var alphaNum = []rune("0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ=")

// randString generates a random string of size n consisting of alphanumeric runes
// if lower is set to true, it only samples from the first 36 runes (numbers and lowercase letters)
func randString(n int, lower bool) string {
	m := len(alphaNum)
	if lower {
		m = 36
	}

	s := make([]rune, n)
	for i := range s {
		s[i] = alphaNum[rand.Intn(m)]
	}

	return string(s)
}

// CreateNew provisions a new service account key,
// returns the key-id and private-key data of the created key if successful,
// otherwise returns error
func (p *MockSvcProvisioner) CreateNew(labels map[string]string) (string, []byte, error) {
	return randString(40, true), []byte(randString(3096, false)), nil
}

// Deactivate deletes an existing service account key specified by labels and version,
// returns nil if successful, otherwise error
func (p *MockSvcProvisioner) Deactivate(labels map[string]string, version string) error {
	return nil
}
