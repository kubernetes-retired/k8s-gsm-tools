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

// RotatedSecretConfig contains the slice of RotatedSecretSpecs
type RotatedSecretConfig struct {
	Specs []RotatedSecretSpec `yaml:"specs"`
}

// RotatedSecretSpec specifies a single rotated secret
type RotatedSecretSpec struct {
	Project     string            `yaml:"project"`
	Secret      string            `yaml:"secret"`
	Type        RotatedSecretType `yaml:"type"`
	Refresh     RefreshStrategy   `yaml:"refreshStrategy"`
	GracePeriod time.Duration     `yaml:"gracePeriod"`
}

// RotatedSecretType specifies the type of the rotated secret
// One and only one of its fields can be assigned a value
// others should be set to nil
type RotatedSecretType struct {
	ServiceAccountKey *svckey.ServiceAccountKeySpec `yaml:"serviceAccountKey,omitempty"`
}

// RefreshStrategy specifies the refeshing strategy for the rotated secret
// One and only one of its fields can be assigned a value
// others should be set to nil
type RefreshStrategy struct {
	Interval time.Duration `yaml:"interval,omitempty"`
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
		return "UNKNOWN"
	}
}

// RotatedSecretType.Labels() is used to obtain the labels needed for the provisioner
func (secretType RotatedSecretType) Labels() map[string]string {
	if secretType.ServiceAccountKey != nil {
		return secretType.ServiceAccountKey.Labels()
	} else {
		// TODO: other types of secrets
		return nil
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

	for _, spec := range config.Specs {
		switch {
		case spec.Project == "":
			return fmt.Errorf("Missing <project> field for rotated secret: %s.", spec)
		case spec.Secret == "":
			return fmt.Errorf("Missing <secret> field for rotated secret: %s.", spec)
		}

		// validate there's only one refresh stategy
		// TODO: modify this after other refresh strategies are supported
		if spec.Refresh.Interval == 0 {
			return fmt.Errorf("Missing <refresh strategy> for rotated secret: %s.", spec)
		}

		// validate there's only one secret type
		// TODO: modify this after other types are supported
		if spec.Type.ServiceAccountKey == nil {
			return fmt.Errorf("Missing <type> for rotated secret: %s.", spec)
		}
	}
	return nil
}
