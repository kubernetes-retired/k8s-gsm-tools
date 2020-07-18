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

package controller

import (
	"bytes"
	"context"
	"flag"
	"fmt"
	"github.com/b01901143/secret-sync-controller/pkg/client"
	"github.com/b01901143/secret-sync-controller/pkg/config"
	"github.com/b01901143/secret-sync-controller/tests"
	"os"
	"testing"
	"text/template"
)

var testClient tests.ClientInterface

type testOptions struct {
	e2eClient  bool
	gsmProject string
	kubeconfig string
}

var testOpts testOptions

func TestMain(m *testing.M) {
	flag.BoolVar(&testOpts.e2eClient, "e2e-client", false, "Test with API or mock client.")
	flag.StringVar(&testOpts.gsmProject, "gsm-project", "project-1", "Secret MAnager prject for e2e testing.")
	flag.StringVar(&testOpts.kubeconfig, "kubeconfig", "", "Path to kubeconfig file.")
	flag.Parse()

	if !testOpts.e2eClient {
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

type KubernetesSecret struct {
	Namespace string
	Secret    string
	Key       string
	Value     string
}

func TestSync(t *testing.T) {
	var fixtureConfig = `
      secretmanager:
        {{.}}: 
          gsm-password: gsm-password-v1
          gsm-token: gsm-token-v1
          gsm-old-token: old-token
      kubernetes:
        ns-a:
          secret-a:
            key-a: old-token
        ns-b:
          secret-b:
        ns-c:
`
	// substitute fixture project to testOpts.gsmProject
	temp := template.Must(template.New("config").Parse(fixtureConfig))
	fixtureBuffer := new(bytes.Buffer)
	temp.Execute(fixtureBuffer, testOpts.gsmProject)

	fixture, err := tests.NewFixture(fixtureBuffer.Bytes())
	if err != nil {
		t.Fatalf("Fail to parse fixture: %s", err)
	}

	err = fixture.Setup(testClient)
	if err != nil {
		t.Fatalf("Fail to setup fixture: %s", err)
	}

	defer fixture.Teardown(testClient)

	var testcases = []struct {
		name      string
		spec      config.SecretSyncSpec
		want      KubernetesSecret
		update    bool
		expectErr bool
	}{
		{
			name: "Sync from <existing gsm secret> to <existing k8s secret> with <existing key> containing the <same secret values>. Should not update.",
			spec: config.SecretSyncSpec{
				Source: config.SecretManagerSpec{
					Project: testOpts.gsmProject,
					Secret:  "gsm-old-token",
				},
				Destination: config.KubernetesSpec{
					Namespace: "ns-a",
					Secret:    "secret-a",
					Key:       "key-a",
				},
			},

			want: KubernetesSecret{
				Namespace: "ns-a",
				Secret:    "secret-a",
				Key:       "key-a",
				Value:     "old-token",
			},

			update: false,

			expectErr: false,
		},
		{
			name: "Sync from <existing gsm secret> to <existing k8s secret> with <existing key>. Should update the key-value pair.",
			spec: config.SecretSyncSpec{
				Source: config.SecretManagerSpec{
					Project: testOpts.gsmProject,
					Secret:  "gsm-password",
				},
				Destination: config.KubernetesSpec{
					Namespace: "ns-a",
					Secret:    "secret-a",
					Key:       "key-a",
				},
			},

			want: KubernetesSecret{
				Namespace: "ns-a",
				Secret:    "secret-a",
				Key:       "key-a",
				Value:     "gsm-password-v1",
			},

			update: true,

			expectErr: false,
		},
		{
			name: "Sync from <existing gsm secret> to <existing k8s secret> with <non-existing key>. Should insert a new key-value pair.",
			spec: config.SecretSyncSpec{
				Source: config.SecretManagerSpec{
					Project: testOpts.gsmProject,
					Secret:  "gsm-token",
				},
				Destination: config.KubernetesSpec{
					Namespace: "ns-a",
					Secret:    "secret-a",
					Key:       "missed",
				},
			},

			want: KubernetesSecret{
				Namespace: "ns-a",
				Secret:    "secret-a",
				Key:       "missed",
				Value:     "gsm-token-v1",
			},

			update: true,

			expectErr: false,
		},
		{
			name: "Sync from <existing gsm secret> to <existing empty k8s secret> with <no key>. Should insert a new key-value pair.",
			spec: config.SecretSyncSpec{
				Source: config.SecretManagerSpec{
					Project: testOpts.gsmProject,
					Secret:  "gsm-token",
				},
				Destination: config.KubernetesSpec{
					Namespace: "ns-b",
					Secret:    "secret-b",
					Key:       "missed",
				},
			},

			want: KubernetesSecret{
				Namespace: "ns-b",
				Secret:    "secret-b",
				Key:       "missed",
				Value:     "gsm-token-v1",
			},

			update: true,

			expectErr: false,
		},
		{
			name: "Sync from <existing gsm secret> to <non-existing k8s secret> in <existing k8s namespace>. Should insert a new secret with the key-value pair.",
			spec: config.SecretSyncSpec{
				Source: config.SecretManagerSpec{
					Project: testOpts.gsmProject,
					Secret:  "gsm-token",
				},
				Destination: config.KubernetesSpec{
					Namespace: "ns-c",
					Secret:    "missed",
					Key:       "missed",
				},
			},

			want: KubernetesSecret{
				Namespace: "ns-c",
				Secret:    "missed",
				Key:       "missed",
				Value:     "gsm-token-v1",
			},

			update: true,

			expectErr: false,
		},
		{
			name: "Sync from <existing gsm secret> to <k8s secret> in <non-existing k8s namespace>. Should return error.",
			spec: config.SecretSyncSpec{
				Source: config.SecretManagerSpec{
					Project: testOpts.gsmProject,
					Secret:  "gsm-token",
				},
				Destination: config.KubernetesSpec{
					Namespace: "missed",
					Secret:    "secret-a",
					Key:       "key-a",
				},
			},

			want: KubernetesSecret{},

			update: false,

			expectErr: true,
		},
		{
			name: "Sync from <non-existing gsm secret>. Should return error.",
			spec: config.SecretSyncSpec{
				Source: config.SecretManagerSpec{
					Project: testOpts.gsmProject,
					Secret:  "missed",
				},
				Destination: config.KubernetesSpec{
					Namespace: "ns-a",
					Secret:    "secret-a",
					Key:       "key-a",
				},
			},

			want: KubernetesSecret{},

			update: false,

			expectErr: true,
		},
	}
	for _, tc := range testcases {
		testname := tc.name
		t.Run(testname, func(t *testing.T) {

			controller := &SecretSyncController{
				Client:  testClient,
				RunOnce: true,
			}

			err = fixture.Reset(testClient)
			if err != nil {
				t.Error(err)
			}

			updated, err := controller.Sync(tc.spec)
			if tc.update && !updated {
				t.Errorf("Expected update in destination secret value.")
			} else if !tc.update && updated {
				t.Errorf("Unexpected update in destination secret value.")
			}

			if tc.expectErr && err == nil {
				t.Errorf("Failed to receive expected error.")
			} else if !tc.expectErr && err != nil {
				t.Errorf("Unexpected error: %s", err)
			}

			if tc.expectErr {
				return
			}

			// validate result
			value, err := controller.Client.GetKubernetesSecretValue(tc.want.Namespace, tc.want.Secret, tc.want.Key)
			if err != nil {
				t.Error(err)
			}
			if !bytes.Equal(value, []byte(tc.want.Value)) {
				t.Errorf("Fail to validate namespaces/%s/secrets/%s[%s]. Expected %s but got %s.", tc.want.Namespace, tc.want.Secret, tc.want.Key, tc.want.Value, value)
			}

		})
	}
}

func TestSyncAll(t *testing.T) {
	var fixtureConfig = `
      secretmanager:
        {{.}}: 
          gsm-password: gsm-password-v1
          gsm-token: gsm-token-v1
          gsm-old-token: old-token
      kubernetes:
        ns-a:
          secret-a:
            key-a: old-token
        ns-b:
          secret-b:
        ns-c:
`
	// substitute fixture project to testOpts.gsmProject
	temp := template.Must(template.New("config").Parse(fixtureConfig))
	fixtureBuffer := new(bytes.Buffer)
	temp.Execute(fixtureBuffer, testOpts.gsmProject)

	fixture, err := tests.NewFixture(fixtureBuffer.Bytes())
	if err != nil {
		t.Fatalf("Fail to parse fixture: %s", err)
	}

	err = fixture.Setup(testClient)
	if err != nil {
		t.Fatalf("Fail to setup fixture: %s", err)
	}

	defer fixture.Teardown(testClient)

	var testcases = []struct {
		name string
		conf config.SecretSyncConfig
		want []KubernetesSecret
	}{
		{
			name: "Sync all pairs normally.",
			conf: config.SecretSyncConfig{
				Specs: []config.SecretSyncSpec{
					{
						Source: config.SecretManagerSpec{
							Project: testOpts.gsmProject,
							Secret:  "gsm-token",
						},
						Destination: config.KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-a",
						},
					},
					{
						Source: config.SecretManagerSpec{
							Project: testOpts.gsmProject,
							Secret:  "gsm-password",
						},
						Destination: config.KubernetesSpec{
							Namespace: "ns-b",
							Secret:    "secret-b",
							Key:       "key-b",
						},
					},
				},
			},

			want: []KubernetesSecret{
				{
					Namespace: "ns-a",
					Secret:    "secret-a",
					Key:       "key-a",
					Value:     "gsm-token-v1",
				},
				{
					Namespace: "ns-b",
					Secret:    "secret-b",
					Key:       "key-b",
					Value:     "gsm-password-v1",
				},
			},
		},
		{
			name: "Sync other pairs normally, and skip <non-existing gsm secret>.",
			conf: config.SecretSyncConfig{
				Specs: []config.SecretSyncSpec{
					{
						Source: config.SecretManagerSpec{
							Project: testOpts.gsmProject,
							Secret:  "missed",
						},
						Destination: config.KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-a",
						},
					},
					{
						Source: config.SecretManagerSpec{
							Project: testOpts.gsmProject,
							Secret:  "gsm-password",
						},
						Destination: config.KubernetesSpec{
							Namespace: "ns-b",
							Secret:    "secret-b",
							Key:       "key-b",
						},
					},
				},
			},

			want: []KubernetesSecret{
				{
					Namespace: "ns-a",
					Secret:    "secret-a",
					Key:       "key-a",
					Value:     "old-token",
				},
				{
					Namespace: "ns-b",
					Secret:    "secret-b",
					Key:       "key-b",
					Value:     "gsm-password-v1",
				},
			},
		},
		{
			name: "Sync other pairs normally, and skip <non-existing k8s namespace>.",
			conf: config.SecretSyncConfig{
				Specs: []config.SecretSyncSpec{
					{
						Source: config.SecretManagerSpec{
							Project: testOpts.gsmProject,
							Secret:  "gsm-token",
						},
						Destination: config.KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-a",
						},
					},
					{
						Source: config.SecretManagerSpec{
							Project: testOpts.gsmProject,
							Secret:  "gsm-password",
						},
						Destination: config.KubernetesSpec{
							Namespace: "missed",
							Secret:    "secret-d",
							Key:       "key-d",
						},
					},
				},
			},

			want: []KubernetesSecret{
				{
					Namespace: "ns-a",
					Secret:    "secret-a",
					Key:       "key-a",
					Value:     "gsm-token-v1",
				},
			},
		},
	}
	for _, tc := range testcases {
		testname := tc.name
		t.Run(testname, func(t *testing.T) {

			controller := &SecretSyncController{
				Client:  testClient,
				Agent:   &config.Agent{},
				RunOnce: true,
			}
			controller.Agent.Set(&tc.conf)

			err = fixture.Reset(testClient)
			if err != nil {
				t.Error(err)
			}

			controller.SyncAll()

			// validate result
			for _, k8sSecret := range tc.want {
				value, err := controller.Client.GetKubernetesSecretValue(k8sSecret.Namespace, k8sSecret.Secret, k8sSecret.Key)
				if err != nil {
					t.Error(err)
				}
				if !bytes.Equal(value, []byte(k8sSecret.Value)) {
					t.Errorf("Fail to validate namespaces/%s/secrets/%s[%s]. Expected %s but got %s.", k8sSecret.Namespace, k8sSecret.Secret, k8sSecret.Key, k8sSecret.Value, value)
				}

			}
		})
	}
}
