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
	"context"
	"flag"
	"fmt"
	"github.com/b01901143/secret-sync-controller/pkg/client"
	"github.com/b01901143/secret-sync-controller/tests"
	"os"
	"reflect"
	"testing"
	"time"
)

var testClient tests.ClientInterface

type testOptions struct {
	e2e        bool
	configPath string
	gsmProject string
	kubeconfig string
}

var testOpts testOptions

func TestMain(m *testing.M) {
	flag.BoolVar(&testOpts.e2e, "e2e", false, "Test with actual ConfigMap.")
	flag.StringVar(&testOpts.configPath, "config-path", "/tmp/config/syncConfig", "Path to mounted ConfigMap.")
	flag.StringVar(&testOpts.gsmProject, "gsm-project", "project-1", "Secret MAnager prject for e2e testing.")
	flag.StringVar(&testOpts.kubeconfig, "kubeconfig", "", "Path to kubeconfig file.")
	flag.Parse()

	if !testOpts.e2e {
		testClient = tests.NewMockClient([]string{testOpts.gsmProject})
	} else {
		// prepare clients
		k8sClientset, err := client.NewK8sClientset(testOpts.kubeconfig)
		if err != nil {
			fmt.Printf("Fail to create new kubernetes client: %s", err)
			os.Exit(1)
		}
		secretManagerClient, err := client.NewSecretManagerClient(context.Background())
		if err != nil {
			fmt.Printf("Fail to create new Secret Manager client: %s", err)
			os.Exit(1)
		}

		testClient = &tests.E2eTestClient{
			&client.Client{
				K8sClientset:        *k8sClientset,
				SecretManagerClient: *secretManagerClient,
			},
		}
	}

	os.Exit(m.Run())
}

func TestWatchConfig(t *testing.T) {
	if !testOpts.e2e {
		return
	}

	var testcases = []struct {
		name          string
		oldConfigFile string
		oldConfig     SecretSyncConfig
		newConfigFile string
		newConfig     SecretSyncConfig
	}{
		{
			name: "Update ConfigMap.",
			oldConfigFile: `
              specs:
              - source: 
                  project: proj-1
                  secret: secret-1
                destination:
                  namespace: ns-1
                  secret: secret-1
                  key: key-1
              - source: 
                  project: proj-1
                  secret: secret-2
                destination:
                  namespace: ns-2
                  secret: secret-2
                  key: key-2
              `,
			oldConfig: SecretSyncConfig{
				Specs: []SecretSyncSpec{
					{
						Source: SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: KubernetesSpec{
							Namespace: "ns-1",
							Secret:    "secret-1",
							Key:       "key-1",
						},
					},
					{
						Source: SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-2",
						},
						Destination: KubernetesSpec{
							Namespace: "ns-2",
							Secret:    "secret-2",
							Key:       "key-2",
						},
					},
				},
			},
			newConfigFile: `
              specs:
              - source: 
                  project: proj-0
                  secret: secret-0
                destination:
                  namespace: ns-0
                  secret: secret-0
                  key: key-0
              `,
			newConfig: SecretSyncConfig{
				Specs: []SecretSyncSpec{
					{
						Source: SecretManagerSpec{
							Project: "proj-0",
							Secret:  "secret-0",
						},
						Destination: KubernetesSpec{
							Namespace: "ns-0",
							Secret:    "secret-0",
							Key:       "key-0",
						},
					},
				},
			},
		},
	}
	for _, tc := range testcases {
		testname := tc.name
		t.Run(testname, func(t *testing.T) {

			configAgent := &Agent{}
			runFunc, err := configAgent.WatchConfig(testOpts.configPath)
			if err != nil {
				t.Error(err)
			}

			if runFunc != nil {
				ctx, cancel := context.WithCancel(context.Background())
				go runFunc(ctx)
				defer cancel()
			}

			// initialize the configMap, and wait for 30 seconds for the mounted configMap to be updated.
			err = testClient.UpsertKubernetesConfigMap("default", "config", "syncConfig", tc.oldConfigFile)
			time.Sleep(30000 * time.Millisecond)

			if err != nil {
				t.Error(err)
			}

			// start a go routine to update configMap, expecting configAgent.Config()
			// to be updated during the first testing loop.
			updateFunc := func() {
				testClient.UpsertKubernetesConfigMap("default", "config", "syncConfig", tc.newConfigFile)
			}
			go updateFunc()

			for i, spec := range configAgent.Config().Specs {
				time.Sleep(35000 * time.Millisecond)
				if i >= len(tc.oldConfig.Specs) {
					t.Errorf("Index out of range [%d] with length %d", i, len(tc.oldConfig.Specs))
					break
				}
				expected := tc.oldConfig.Specs[i]
				if !reflect.DeepEqual(spec, expected) {
					t.Errorf("Mismatched specs before update: %s, %s", spec, expected)
				}
			}

			// 70 seconds after the update of configMap, expect configAgent.Config() to be already updated
			for i, spec := range configAgent.Config().Specs {
				if i >= len(tc.newConfig.Specs) {
					t.Errorf("Index out of range [%d] with length %d", i, len(tc.newConfig.Specs))
					break
				}
				expected := tc.newConfig.Specs[i]
				if !reflect.DeepEqual(spec, expected) {
					t.Errorf("Mismatched specs after update: %s, %s", spec, expected)
				}
			}

		})
	}
}

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
