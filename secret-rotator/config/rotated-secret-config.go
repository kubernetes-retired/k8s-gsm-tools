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
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
	"sigs.k8s.io/k8s-gsm-tools/secret-rotator/svckey"
	"time"
)

// Structs for rotated secret configuration
type RotatedSecretConfig struct {
	Specs []RotatedSecretSpec `yaml:"specs"`
}
type RotatedSecretSpec struct {
	Project     string            `yaml:"project"`
	Secret      string            `yaml:"secret"`
	Type        RotatedSecretType `yaml:"type"`
	Refresh     RefreshStrategy   `yaml:"refreshStrategy"`
	GracePeriod time.Duration     `yaml:"gracePeriod"`
}
type RotatedSecretType struct {
	ServiceAccountKey *svckey.ServiceAccountKeySpec `yaml:"serviceAccountKey,omitempty"`
}
type RefreshStrategy struct {
	Interval time.Duration `yaml:"interval,omitempty"`
	Cron     string        `yaml:"cron,omitempty"`
}

func (config RotatedSecretConfig) String() string {
	d, _ := yaml.Marshal(config)
	return string(d)
}

func (secret RotatedSecretSpec) String() string {
	return fmt.Sprintf("SecretManager:/projects/%s/secrets/%s", secret.Project, secret.Secret)
}

// RotatedSecretType.Type() is used to obtain the provisioner of the type
func (secretType RotatedSecretType) Type() string {
	if secretType.ServiceAccountKey != nil {
		return secretType.ServiceAccountKey.Type()
	} else {
		// TODO: other types of secrets
		return ""
	}
}

// LoadFrom loads the rotated secret configuration from a yaml, returns error if fails.
func (config *RotatedSecretConfig) LoadFrom(file string) error {
	stat, err := os.Stat(file)
	if err != nil {
		return err
	}

	if stat.IsDir() {
		return fmt.Errorf("config cannot be a dir - %s", file)
	}

	yamlFile, err := ioutil.ReadFile(file)
	if err != nil {
		return fmt.Errorf("Error reading %s: %s\n", file, err)
	}

	err = yaml.Unmarshal(yamlFile, config)
	if err != nil {
		return fmt.Errorf("Error unmarshalling %s: %s\n", file, err)
	}

	return nil
}

func (config *RotatedSecretConfig) Validate() error {
	if len(config.Specs) == 0 {
		return fmt.Errorf("Empty secret sync configuration.")
	}
	// TODO: validate Project, Secret, Type, Refresh, GracePeriod fields
	return nil
}
