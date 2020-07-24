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

import (
	"testing"
)

func TestValidate(t *testing.T) {
	var testcases = []struct {
		name      string
		config    SecretSyncConfig
		expectErr bool
	}{
		{
			name: "Correct config",
			config: SecretSyncConfig{
				Specs: []SecretSyncSpec{
					{
						Source: SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-a",
						},
					},
					{
						Source: SecretManagerSpec{
							Project: "proj-2",
							Secret:  "secret-2",
						},
						Destination: KubernetesSpec{
							Namespace: "ns-b",
							Secret:    "secret-b",
							Key:       "key-b",
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Correct config, <Different source secrets> for <two different secret keys< in the <same Kubernetes secret>.",
			config: SecretSyncConfig{
				Specs: []SecretSyncSpec{
					{
						Source: SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-a",
						},
					},
					{
						Source: SecretManagerSpec{
							Project: "proj-2",
							Secret:  "secret-2",
						},
						Destination: KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-b",
						},
					},
				},
			},
			expectErr: false,
		},
		{
			name: "Missing <project> field for <source>.",
			config: SecretSyncConfig{
				Specs: []SecretSyncSpec{
					{
						Source: SecretManagerSpec{
							Secret: "secret-1",
						},
						Destination: KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-a",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Missing <secret> field for <source>.",
			config: SecretSyncConfig{
				Specs: []SecretSyncSpec{
					{
						Source: SecretManagerSpec{
							Project: "proj-1",
						},
						Destination: KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-a",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Missing <namespace> field for <destination>.",
			config: SecretSyncConfig{
				Specs: []SecretSyncSpec{
					{
						Source: SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: KubernetesSpec{
							Secret: "secret-a",
							Key:    "key-a",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Missing <secret> field for <destination>.",
			config: SecretSyncConfig{
				Specs: []SecretSyncSpec{
					{
						Source: SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: KubernetesSpec{
							Namespace: "ns-a",
							Key:       "key-a",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "Missing <key> field for <destination>.",
			config: SecretSyncConfig{
				Specs: []SecretSyncSpec{
					{
						Source: SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "<Multiple sources> for a <single Kunernetes secret key>.",
			config: SecretSyncConfig{
				Specs: []SecretSyncSpec{
					{
						Source: SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-a",
						},
					},
					{
						Source: SecretManagerSpec{
							Project: "proj-2",
							Secret:  "secret-2",
						},
						Destination: KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-a",
						},
					},
				},
			},
			expectErr: true,
		},
		{
			name: "<Multiple declaration> for the <same secret sync pair>.",
			config: SecretSyncConfig{
				Specs: []SecretSyncSpec{
					{
						Source: SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-a",
						},
					},
					{
						Source: SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-a",
						},
					},
				},
			},
			expectErr: true,
		},
	}
	for _, tc := range testcases {
		testname := tc.name
		t.Run(testname, func(t *testing.T) {

			err := tc.config.Validate()
			if tc.expectErr && err == nil {
				t.Errorf("Expected error but got nil.")
			} else if !tc.expectErr && err != nil {
				t.Errorf("Unexpected error: %s", err)
			}

		})
	}
}
