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

// Package config defines configuration and sync-pair structs

import (
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"os"
)

// Structs for secret sync configuration
type SecretSyncConfig struct {
	Specs []SecretSyncSpec `yaml:"specs"`
}

type SecretSyncSpec struct {
	Source      SecretManagerSpec `yaml:"source"`
	Destination KubernetesSpec    `yaml:"destination"`
}

type KubernetesSpec struct {
	Namespace string `yaml:"namespace"`
	Secret    string `yaml:"secret"`
	Key       string `yaml:"key"`
}

type SecretManagerSpec struct {
	Project string `yaml:"project"`
	Secret  string `yaml:"secret"`
}

func (config SecretSyncConfig) String() string {
	d, _ := yaml.Marshal(config)
	return string(d)
}
func (spec SecretSyncSpec) String() string {
	return fmt.Sprintf("{%s -> %s}", spec.Source, spec.Destination)
}
func (gsm SecretManagerSpec) String() string {
	return fmt.Sprintf("SecretManager:/projects/%s/secrets/%s", gsm.Project, gsm.Secret)
}
func (k8s KubernetesSpec) String() string {
	return fmt.Sprintf("Kubernetes:/namespaces/%s/secrets/%s[%s]", k8s.Namespace, k8s.Secret, k8s.Key)
}

// LoadFrom loads the secret sync configuration from a yaml, returns error if fails.
func (config *SecretSyncConfig) LoadFrom(file string) error {
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

func (config *SecretSyncConfig) Validate() error {
	if len(config.Specs) == 0 {
		return fmt.Errorf("Empty secret sync configuration.")
	}
	syncFrom := make(map[KubernetesSpec]SecretManagerSpec)
	for _, spec := range config.Specs {
		switch {
		case spec.Source.Project == "":
			return fmt.Errorf("Missing <project> field for <source> in spec %s.", spec)
		case spec.Source.Secret == "":
			return fmt.Errorf("Missing <secret> field for <source> in spec %s.", spec)
		case spec.Destination.Namespace == "":
			return fmt.Errorf("Missing <namespace> field for <destination> in spec %s.", spec)
		case spec.Destination.Secret == "":
			return fmt.Errorf("Missing <secret> field for <destination> in spec %s.", spec)
		case spec.Destination.Key == "":
			return fmt.Errorf("Missing <key> field for <destination> in spec %s.", spec)
		}

		// check if spec.Destination already has a source
		src, ok := syncFrom[spec.Destination]
		if ok {
			return fmt.Errorf("Fail to generate sync pair %s: Secret %s already has a source (%s).", spec, spec.Destination, src)
		}
		syncFrom[spec.Destination] = spec.Source
	}
	return nil
}
