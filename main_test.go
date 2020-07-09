package main

import (
	"b01901143.git/secret-sync-controller/client"
	"b01901143.git/secret-sync-controller/config"
	"b01901143.git/secret-sync-controller/controller"
	"b01901143.git/secret-sync-controller/tests"
	"bytes"
	"context"
	"flag"
	"fmt"
	"os"
	"testing"
)

var testClient tests.ClientInterface

type testOptions struct {
	e2eClient  bool
	gsmProject string
}

var testOpts testOptions

func TestMain(m *testing.M) {
	flag.BoolVar(&testOpts.e2eClient, "e2e-client", false, "Test with API or mock client.")
	flag.StringVar(&testOpts.gsmProject, "gsm-project", "project-1", "Secret MAnager prject for e2e testing.")
	flag.Parse()

	if !testOpts.e2eClient {
		testClient = tests.NewMockClient([]string{testOpts.gsmProject})
	} else {
		// prepare clients
		k8sClientset, err := client.NewK8sClientset()
		if err != nil {
			fmt.Printf("New kubernetes client failed: %s", err)
			os.Exit(1)
		}
		secretManagerClient, err := client.NewSecretManagerClient(context.Background())
		if err != nil {
			fmt.Printf("New Secret Manager client failed: %s", err)
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

var fixtureConfig = []byte(`
{
  "secretManager": {
    "testOpts.gsmProject": {
      "gsm-password": "gsm-password-v1",
      "gsm-token": "gsm-token-v1",
      "gsm-old-token": "old-token"
    }
  },
  "kubernetes": {
    "ns-a": {
      "secret-a": {
        "key-a": "old-token"
      }
    },
    "ns-b": {
      "secret-b": {}
    },
    "ns-c": {}
  }
}
`)

func TestSync(t *testing.T) {
	fixture, err := tests.NewFixture(fixtureConfig, testOpts.gsmProject)
	if err != nil {
		t.Fatalf("Fail to parse fixture: %s", err)
	}

	var testcases = []struct {
		name      string
		spec      config.SecretSyncSpec
		want      tests.Fixture
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

			want: tests.Fixture{
				"ns-a": map[string]interface{}{
					"secret-a": map[string]string{
						"key-a": "old-token",
					},
				},
				"ns-b": map[string]interface{}{
					"secret-b": nil,
				},
				"ns-c": nil,
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

			want: tests.Fixture{
				"ns-a": map[string]interface{}{
					"secret-a": map[string]string{
						"key-a": "gsm-password-v1",
					},
				},
				"ns-b": map[string]interface{}{
					"secret-b": nil,
				},
				"ns-c": nil,
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

			want: tests.Fixture{
				"ns-a": map[string]interface{}{
					"secret-a": map[string]string{
						"key-a":  "old-token",
						"missed": "gsm-token-v1",
					},
				},
				"ns-b": map[string]interface{}{
					"secret-b": nil,
				},
				"ns-c": nil,
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

			want: tests.Fixture{
				"ns-a": map[string]interface{}{
					"secret-a": map[string]string{
						"key-a": "old-token",
					},
				},
				"ns-b": map[string]interface{}{
					"secret-b": map[string]string{
						"missed": "gsm-token-v1",
					},
				},
				"ns-c": nil,
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

			want: tests.Fixture{
				"ns-a": map[string]interface{}{
					"secret-a": map[string]string{
						"key-a": "old-token",
					},
				},
				"ns-b": map[string]interface{}{
					"secret-b": nil,
				},
				"ns-c": map[string]interface{}{
					"missed": map[string]string{
						"missed": "gsm-token-v1",
					},
				},
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

			want: tests.Fixture{
				"ns-a": map[string]interface{}{
					"secret-a": map[string]string{
						"key-a": "old-token",
					},
				},
				"ns-b": map[string]interface{}{
					"secret-b": nil,
				},
				"ns-c": nil,
			},

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

			want: tests.Fixture{
				"ns-a": map[string]interface{}{
					"secret-a": map[string]string{
						"key-a": "old-token",
					},
				},
				"ns-b": map[string]interface{}{
					"secret-b": nil,
				},
				"ns-c": nil,
			},

			update: false,

			expectErr: true,
		},
	}
	for _, tc := range testcases {
		testname := tc.name
		t.Run(testname, func(t *testing.T) {

			controller := &controller.SecretSyncController{
				Client:  testClient,
				RunOnce: true,
			}

			err = fixture.Setup(testClient)
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
				t.Errorf("Expected error but got nil.")
			} else if !tc.expectErr && err != nil {
				t.Errorf("Unexpected error: %s", err)
			}

			// validate result
			for namespace, nsItem := range tc.want {
				if nsItem == nil {
					continue
				}
				for secret, secretItem := range nsItem.(map[string]interface{}) {
					err = controller.Client.ValidateKubernetesSecret(namespace, secret)
					if err != nil {
						t.Error(err)
					}

					if secretItem == nil {
						continue
					}
					for key, data := range secretItem.(map[string]string) {
						value, err := controller.Client.GetKubernetesSecretValue(namespace, secret, key)
						if err != nil {
							t.Error(err)
						}
						if !bytes.Equal(value, []byte(data)) {
							t.Errorf("Fail to validate namespaces/%s/secrets/%s[%s]. Expected %s but got %s.", namespace, secret, key, data, value)
						}

					}
				}
			}

			err = fixture.Teardown(testClient)
			if err != nil {
				t.Error(err)
			}
		})
	}
}

func TestValidate(t *testing.T) {
	var testcases = []struct {
		name      string
		conf      config.SecretSyncConfig
		expectErr bool
	}{
		{
			name: "Correct config.",
			conf: config.SecretSyncConfig{
				Specs: []config.SecretSyncSpec{
					{
						Source: config.SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: config.KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-a",
						},
					},
					{
						Source: config.SecretManagerSpec{
							Project: "proj-2",
							Secret:  "secret-2",
						},
						Destination: config.KubernetesSpec{
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
			name: "Correct config. <Different source secrets> for <two different secret keys< in the <same Kubernetes secret>.",
			conf: config.SecretSyncConfig{
				Specs: []config.SecretSyncSpec{
					{
						Source: config.SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: config.KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-a",
						},
					},
					{
						Source: config.SecretManagerSpec{
							Project: "proj-2",
							Secret:  "secret-2",
						},
						Destination: config.KubernetesSpec{
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
			conf: config.SecretSyncConfig{
				Specs: []config.SecretSyncSpec{
					{
						Source: config.SecretManagerSpec{
							Secret: "secret-1",
						},
						Destination: config.KubernetesSpec{
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
			conf: config.SecretSyncConfig{
				Specs: []config.SecretSyncSpec{
					{
						Source: config.SecretManagerSpec{
							Project: "proj-1",
						},
						Destination: config.KubernetesSpec{
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
			conf: config.SecretSyncConfig{
				Specs: []config.SecretSyncSpec{
					{
						Source: config.SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: config.KubernetesSpec{
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
			conf: config.SecretSyncConfig{
				Specs: []config.SecretSyncSpec{
					{
						Source: config.SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: config.KubernetesSpec{
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
			conf: config.SecretSyncConfig{
				Specs: []config.SecretSyncSpec{
					{
						Source: config.SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: config.KubernetesSpec{
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
			conf: config.SecretSyncConfig{
				Specs: []config.SecretSyncSpec{
					{
						Source: config.SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: config.KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-a",
						},
					},
					{
						Source: config.SecretManagerSpec{
							Project: "proj-2",
							Secret:  "secret-2",
						},
						Destination: config.KubernetesSpec{
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
			conf: config.SecretSyncConfig{
				Specs: []config.SecretSyncSpec{
					{
						Source: config.SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: config.KubernetesSpec{
							Namespace: "ns-a",
							Secret:    "secret-a",
							Key:       "key-a",
						},
					},
					{
						Source: config.SecretManagerSpec{
							Project: "proj-1",
							Secret:  "secret-1",
						},
						Destination: config.KubernetesSpec{
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

			err := tc.conf.Validate()
			if tc.expectErr && err == nil {
				t.Errorf("Expected error but got nil.")
			} else if !tc.expectErr && err != nil {
				t.Errorf("Unexpected error: %s", err)
			}

		})
	}
}

/*
# fixtureConfig.json
{
  "secretManager": {
    testOpts.gsmProject: {
      "gsm-password": "gsm-password-v1",
      "gsm-token": "gsm-token-v1",
      "gsm-old-token": "old-token"
    }
  },
  "kubernetes": {
    "ns-a": {
      "secret-a": {
        "key-a": "old-token"
      }
    },
    "ns-b": {
      "secret-b": {}
    },
    "ns-c": {}
  }
}

*/

func TestSyncAll(t *testing.T) {
	fixture, err := tests.NewFixture(fixtureConfig, testOpts.gsmProject)
	if err != nil {
		t.Fatalf("Fail to parse fixture: %s", err)
	}

	var testcases = []struct {
		name           string
		conf           config.SecretSyncConfig
		resBefore      tests.Fixture
		fixSource      tests.Fixture
		fixDestination tests.Fixture
		resAfter       tests.Fixture
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

			resBefore: tests.Fixture{
				"ns-a": map[string]interface{}{
					"secret-a": map[string]string{
						"key-a": "gsm-token-v1",
					},
				},
				"ns-b": map[string]interface{}{
					"secret-b": map[string]string{
						"key-b": "gsm-password-v1",
					},
				},
				"ns-c": nil,
			},

			fixSource: nil,

			fixDestination: nil,

			resAfter: tests.Fixture{
				"ns-a": map[string]interface{}{
					"secret-a": map[string]string{
						"key-a": "gsm-token-v1",
					},
				},
				"ns-b": map[string]interface{}{
					"secret-b": map[string]string{
						"key-b": "gsm-password-v1",
					},
				},
				"ns-c": nil,
			},
		},
		{
			name: "Sync other pairs normally, and recover from <non-existing gsm secret> whenever availalbe.",
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

			resBefore: tests.Fixture{
				"ns-a": map[string]interface{}{
					"secret-a": map[string]string{
						"key-a": "old-token",
					},
				},
				"ns-b": map[string]interface{}{
					"secret-b": map[string]string{
						"key-b": "gsm-password-v1",
					},
				},
				"ns-c": nil,
			},

			fixSource: tests.Fixture{
				testOpts.gsmProject: map[string]string{
					"missed": "new-secret",
				},
			},

			fixDestination: nil,

			resAfter: tests.Fixture{
				"ns-a": map[string]interface{}{
					"secret-a": map[string]string{
						"key-a": "new-secret",
					},
				},
				"ns-b": map[string]interface{}{
					"secret-b": map[string]string{
						"key-b": "gsm-password-v1",
					},
				},
				"ns-c": nil,
			},
		},
		{
			name: "Sync other pairs normally, and recover from <non-existing k8s namespace> whenever availalbe.",
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

			resBefore: tests.Fixture{
				"ns-a": map[string]interface{}{
					"secret-a": map[string]string{
						"key-a": "gsm-token-v1",
					},
				},
				"ns-b": map[string]interface{}{
					"secret-b": nil,
				},
				"ns-c": nil,
			},

			fixSource: nil,

			fixDestination: tests.Fixture{
				"missed": nil,
			},

			resAfter: tests.Fixture{
				"ns-a": map[string]interface{}{
					"secret-a": map[string]string{
						"key-a": "gsm-token-v1",
					},
				},
				"ns-b": map[string]interface{}{
					"secret-b": nil,
				},
				"ns-c": nil,
				"missed": map[string]interface{}{
					"secret-d": map[string]string{
						"key-d": "gsm-password-v1",
					},
				},
			},
		},
	}
	for _, tc := range testcases {
		testname := tc.name
		t.Run(testname, func(t *testing.T) {

			controller := &controller.SecretSyncController{
				Client:  testClient,
				Config:  &tc.conf,
				RunOnce: true,
			}

			err = fixture.Setup(testClient)
			if err != nil {
				t.Error(err)
			}

			controller.SyncAll()

			// validate result before recovery
			for namespace, nsItem := range tc.resBefore {
				// check if the namespace exists
				err := controller.Client.ValidateKubernetesNamespace(namespace)
				if err != nil {
					t.Error(err)
				}

				if nsItem == nil {
					continue
				}
				for secret, secretItem := range nsItem.(map[string]interface{}) {
					err = controller.Client.ValidateKubernetesSecret(namespace, secret)
					if err != nil {
						t.Error(err)
					}

					if secretItem == nil {
						continue
					}
					for key, data := range secretItem.(map[string]string) {
						value, err := controller.Client.GetKubernetesSecretValue(namespace, secret, key)
						if err != nil {
							t.Error(err)
						}
						if !bytes.Equal(value, []byte(data)) {
							t.Errorf("Fail to validate namespaces/%s/secrets/%s[%s]. Expected %s but got %s.", namespace, secret, key, data, value)
						}

					}
				}
			}

			for project, projItem := range tc.fixSource {
				for id, data := range projItem.(map[string]string) {
					err = controller.Client.UpsertSecretManagerSecret(project, id, []byte(data))
					if err != nil {
						t.Error(err)
					}

					// record the created secret in fixture for future teardown
					fixture["secretManager"].(map[string]interface{})[project].(map[string]interface{})[id] = data
				}
			}
			for namespace, _ := range tc.fixDestination {
				err = controller.Client.CreateKubernetesNamespace(namespace)
				if err != nil {
					t.Error(err)
				}

				// record the created namespace in fixture for future teardown
				fixture["kubernetes"].(map[string]interface{})[namespace] = nil
			}

			controller.SyncAll()

			// validate result after recovery
			for namespace, nsItem := range tc.resAfter {
				// check if the namespace exists
				err := controller.Client.ValidateKubernetesNamespace(namespace)
				if err != nil {
					t.Error(err)
				}

				if nsItem == nil {
					continue
				}
				for secret, secretItem := range nsItem.(map[string]interface{}) {
					err = controller.Client.ValidateKubernetesSecret(namespace, secret)
					if err != nil {
						t.Error(err)
					}

					if secretItem == nil {
						continue
					}
					for key, data := range secretItem.(map[string]string) {
						value, err := controller.Client.GetKubernetesSecretValue(namespace, secret, key)
						if err != nil {
							t.Error(err)
						}
						if !bytes.Equal(value, []byte(data)) {
							t.Errorf("Fail to validate namespaces/%s/secrets/%s[%s]. Expected %s but got %s.", namespace, secret, key, data, value)
						}

					}
				}
			}

			err = fixture.Teardown(testClient)
			if err != nil {
				t.Error(err)
			}
		})
	}
}
