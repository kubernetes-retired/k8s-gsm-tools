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

package rotator

import (
	"bytes"
	"math/rand"
	"reflect"
	"sigs.k8s.io/k8s-gsm-tools/secret-rotator/config"
	"sigs.k8s.io/k8s-gsm-tools/secret-rotator/svckey"
	"sigs.k8s.io/k8s-gsm-tools/secret-rotator/tests"
	"testing"
	"time"

	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

var str2Time = func(str string) time.Time {
	t, _ := time.Parse(time.RFC3339, str)
	return t
}
var str2Duration = func(str string) time.Duration {
	d, _ := time.ParseDuration(str)
	return d
}

func TestRefresh(t *testing.T) {

	// prepare provisioners for all supported types of secrets
	provisioners := map[string]SecretProvisioner{}
	provisioners[svckey.ServiceAccountKeySpec{}.Type()] = &tests.MockSvcProvisioner{}

	rotator := &SecretRotator{
		Provisioners: provisioners,
	}

	var testcases = []struct {
		name           string
		client         *tests.MockClient
		spec           config.RotatedSecretSpec
		now            time.Time
		refresh        bool
		expectedLabels map[string]string
		expectVerNum   string
		expectErr      bool
	}{
		{
			name: "Within refresh interval. Should not refresh secret.",

			client: &tests.MockClient{
				Secrets: map[string]map[string]*tests.Secret{
					"project-1": map[string]*tests.Secret{
						"secret-1": &tests.Secret{
							Versions: map[string]*tests.Version{
								"1": &tests.Version{
									CreateTime: str2Time("2000-01-01T00:00:00+00:00"),
									Data:       []byte("secret-data-1"),
									State:      secretmanagerpb.SecretVersion_ENABLED,
								},
							},
							Labels: map[string]string{
								"project":         "project-1",
								"service-account": "service-foo",
								"v1":              "key_id-1",
							},
						},
					},
				},
			},

			spec: config.RotatedSecretSpec{
				Project: "project-1",
				Secret:  "secret-1",
				Type: config.RotatedSecretType{
					ServiceAccountKey: &svckey.ServiceAccountKeySpec{
						Project:        "project-1",
						ServiceAccount: "service-foo",
					},
				},
				Refresh: config.RefreshStrategy{
					Interval: str2Duration("20h"),
				},
			},

			now: str2Time("2000-01-01T16:00:00+00:00"),

			refresh: false,

			expectedLabels: map[string]string{
				"project":         "project-1",
				"service-account": "service-foo",
				"v1":              "key_id-1",
			},

			expectVerNum: "1",

			expectErr: false,
		},
		{
			name: "Out of refresh interval. Should refresh secret.",

			client: &tests.MockClient{
				Secrets: map[string]map[string]*tests.Secret{
					"project-1": map[string]*tests.Secret{
						"secret-1": &tests.Secret{
							Versions: map[string]*tests.Version{
								"1": &tests.Version{
									CreateTime: str2Time("2000-01-01T00:00:00+00:00"),
									Data:       []byte("secret-data-1"),
									State:      secretmanagerpb.SecretVersion_ENABLED,
								},
							},
							Labels: map[string]string{
								"project":         "project-1",
								"service-account": "service-foo",
								"v1":              "key_id-1",
							},
						},
					},
				},
			},

			spec: config.RotatedSecretSpec{
				Project: "project-1",
				Secret:  "secret-1",
				Type: config.RotatedSecretType{
					ServiceAccountKey: &svckey.ServiceAccountKeySpec{
						Project:        "project-1",
						ServiceAccount: "service-foo",
					},
				},
				Refresh: config.RefreshStrategy{
					Interval: str2Duration("15h"),
				},
			},

			now: str2Time("2000-01-01T16:00:00+00:00"),

			refresh: true,

			expectedLabels: map[string]string{
				"project":         "project-1",
				"service-account": "service-foo",
				"v1":              "key_id-1",
			},

			expectVerNum: "2",

			expectErr: false,
		},
		{
			name: "Non-existing secret. Should upsert secret version.",

			client: &tests.MockClient{
				Secrets: map[string]map[string]*tests.Secret{
					"project-1": map[string]*tests.Secret{
						"secret-1": &tests.Secret{
							Versions: map[string]*tests.Version{},
							Labels:   map[string]string{},
						},
					},
				},
			},

			spec: config.RotatedSecretSpec{
				Project: "project-1",
				Secret:  "missed",
				Type: config.RotatedSecretType{
					ServiceAccountKey: &svckey.ServiceAccountKeySpec{
						Project:        "project-1",
						ServiceAccount: "service-foo",
					},
				},
				Refresh: config.RefreshStrategy{
					Interval: str2Duration("15h"),
				},
			},

			now: str2Time("2000-01-01T16:00:00+00:00"),

			expectVerNum: "1",

			expectErr: true,
		},
		{
			name: "Non-existing project. Should return error.",

			client: &tests.MockClient{
				Secrets: map[string]map[string]*tests.Secret{
					"project-1": map[string]*tests.Secret{
						"secret-1": &tests.Secret{
							Versions: map[string]*tests.Version{
								"1": &tests.Version{
									CreateTime: str2Time("2000-01-01T00:00:00+00:00"),
									Data:       []byte("secret-data-1"),
									State:      secretmanagerpb.SecretVersion_ENABLED,
								},
							},
							Labels: map[string]string{
								"project":         "project-1",
								"service-account": "service-foo",
								"v1":              "key_id-1",
							},
						},
					},
				},
			},

			spec: config.RotatedSecretSpec{
				Project: "missed",
				Secret:  "missed",
				Type: config.RotatedSecretType{
					ServiceAccountKey: &svckey.ServiceAccountKeySpec{
						Project:        "project-1",
						ServiceAccount: "service-foo",
					},
				},
				Refresh: config.RefreshStrategy{
					Interval: str2Duration("15h"),
				},
			},

			now: str2Time("2000-01-01T16:00:00+00:00"),

			expectErr: true,
		},
	}
	for _, tc := range testcases {
		testname := tc.name
		rotator.Client = tc.client

		t.Run(testname, func(t *testing.T) {
			seed := time.Now().UnixNano()
			rand.Seed(seed)

			refreshed, err := rotator.Refresh(tc.spec, nil, tc.now)
			if tc.refresh && !refreshed {
				t.Errorf("Expected %s to be refreshed.", tc.spec)
			} else if !tc.refresh && refreshed {
				t.Errorf("Unexpected refresh for %s.", tc.spec)
			}

			if tc.expectErr && err == nil {
				t.Errorf("Failed to receive expected error.")
			} else if !tc.expectErr && err != nil {
				t.Errorf("Unexpected error: %s", err)
			}

			if tc.expectErr {
				return
			}

			if refreshed {
				// obtain provisioned data with the same seed
				rand.Seed(seed)
				newSecretKey, newSecretValue, _ := rotator.Provisioners[tc.spec.Type.Type()].CreateNew(nil)

				// insert label for the latest key
				tc.expectedLabels["v"+tc.expectVerNum] = newSecretKey

				value, err := rotator.Client.GetSecretVersionData(tc.spec.Project, tc.spec.Secret, "latest")
				if err != nil {
					t.Error(err)
				}

				if !bytes.Equal(value, newSecretValue) {
					t.Errorf("Fail to validate refreshed secret value of %s. Expected %s but got %s.", tc.spec, newSecretValue, value)
				}
			}

			labels, err := rotator.Client.GetSecretLabels(tc.spec.Project, tc.spec.Secret)
			if err != nil {
				t.Error(err)
			}

			if !reflect.DeepEqual(labels, tc.expectedLabels) {
				t.Errorf("Fail to validate refreshed secret labels of %s. Expected %s but got %s.", tc.spec, tc.expectedLabels, labels)
			}

		})
	}
}

func TestDeactivate(t *testing.T) {

	// prepare provisioners for all supported types of secrets
	provisioners := map[string]SecretProvisioner{}
	provisioners[svckey.ServiceAccountKeySpec{}.Type()] = &tests.MockSvcProvisioner{}

	rotator := &SecretRotator{
		Provisioners: provisioners,
	}

	var testcases = []struct {
		name           string
		client         *tests.MockClient
		spec           config.RotatedSecretSpec
		now            time.Time
		deactiveVers   []string
		expectedLabels map[string]string
	}{
		{
			name: "v3 is within gracePeriod; v1 and v2 are out of gracePeriod. Should deactivate only v1 and v2.",

			client: &tests.MockClient{
				Secrets: map[string]map[string]*tests.Secret{
					"project-1": map[string]*tests.Secret{
						"secret-1": &tests.Secret{
							Versions: map[string]*tests.Version{
								"1": &tests.Version{
									CreateTime: str2Time("2000-01-01T00:00:00+00:00"),
									Data:       []byte("secret-data-1"),
									State:      secretmanagerpb.SecretVersion_ENABLED,
								},
								"2": &tests.Version{
									CreateTime: str2Time("2000-01-01T07:00:00+00:00"),
									Data:       []byte("secret-data-2"),
									State:      secretmanagerpb.SecretVersion_ENABLED,
								},
								"3": &tests.Version{
									CreateTime: str2Time("2000-01-01T14:00:00+00:00"),
									Data:       []byte("secret-data-3"),
									State:      secretmanagerpb.SecretVersion_ENABLED,
								},
								"4": &tests.Version{
									CreateTime: str2Time("2000-01-01T21:00:00+00:00"),
									Data:       []byte("secret-data-4"),
									State:      secretmanagerpb.SecretVersion_ENABLED,
								},
							},
							Labels: map[string]string{
								"project":         "project-1",
								"service-account": "service-foo",
								"v1":              "key_id-1",
								"v2":              "key_id-2",
								"v3":              "key_id-3",
								"v4":              "key_id-4",
							},
						},
					},
				},
			},

			spec: config.RotatedSecretSpec{
				Project: "project-1",
				Secret:  "secret-1",
				Type: config.RotatedSecretType{
					ServiceAccountKey: &svckey.ServiceAccountKeySpec{
						Project:        "project-1",
						ServiceAccount: "service-foo",
					},
				},
				GracePeriod: str2Duration("2h"),
			},

			now: str2Time("2000-01-01T22:00:00+00:00"),

			deactiveVers: []string{"1", "2"},

			expectedLabels: map[string]string{
				"project":         "project-1",
				"service-account": "service-foo",
				"v3":              "key_id-3",
				"v4":              "key_id-4",
			},
		},
		{
			name: "GSM has a label of v3 while version 3 does not exist. Should pop error for v3 and still deactivate v1.",

			client: &tests.MockClient{
				Secrets: map[string]map[string]*tests.Secret{
					"project-1": map[string]*tests.Secret{
						"secret-1": &tests.Secret{
							Versions: map[string]*tests.Version{
								"1": &tests.Version{
									CreateTime: str2Time("2000-01-01T00:00:00+00:00"),
									Data:       []byte("secret-data-1"),
									State:      secretmanagerpb.SecretVersion_ENABLED,
								},
								"2": &tests.Version{
									CreateTime: str2Time("2000-01-01T07:00:00+00:00"),
									Data:       []byte("secret-data-2"),
									State:      secretmanagerpb.SecretVersion_ENABLED,
								},
							},
							Labels: map[string]string{
								"project":         "project-1",
								"service-account": "service-foo",
								"v1":              "key_id-1",
								"v2":              "key_id-2",
								"v3":              "_",
							},
						},
					},
				},
			},

			spec: config.RotatedSecretSpec{
				Project: "project-1",
				Secret:  "secret-1",
				Type: config.RotatedSecretType{
					ServiceAccountKey: &svckey.ServiceAccountKeySpec{
						Project:        "project-1",
						ServiceAccount: "service-foo",
					},
				},
				GracePeriod: str2Duration("2h"),
			},

			now: str2Time("2000-01-01T22:00:00+00:00"),

			deactiveVers: []string{"1"},

			expectedLabels: map[string]string{
				"project":         "project-1",
				"service-account": "service-foo",
				"v2":              "key_id-2",
				"v3":              "_",
			},
		},
	}
	for _, tc := range testcases {
		testname := tc.name
		rotator.Client = tc.client

		t.Run(testname, func(t *testing.T) {
			rotator.Deactivate(tc.spec, tc.now)

			for _, version := range tc.deactiveVers {
				state, err := rotator.Client.GetSecretVersionState(tc.spec.Project, tc.spec.Secret, version)
				if err != nil {
					t.Error(err)
				}

				if state != secretmanagerpb.SecretVersion_DESTROYED {
					t.Errorf("Fail to validate state of %s/versions/%s. Expected %s but got %s.", tc.spec, version, secretmanagerpb.SecretVersion_DESTROYED, state)
				}
			}

			labels, err := rotator.Client.GetSecretLabels(tc.spec.Project, tc.spec.Secret)
			if err != nil {
				t.Error(err)
			}

			if !reflect.DeepEqual(labels, tc.expectedLabels) {
				t.Errorf("Fail to validate refreshed secret labels of %s. Expected %s but got %s.", tc.spec, tc.expectedLabels, labels)
			}

		})
	}
}
