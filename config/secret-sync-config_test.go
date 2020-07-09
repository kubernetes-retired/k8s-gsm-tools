package config

import (
	"b01901143.git/secret-sync-controller/config"
	"testing"
)

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
