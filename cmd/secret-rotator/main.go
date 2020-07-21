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

package main

import (
	"context"
	"flag"
	"fmt"
	"k8s.io/klog"
	"sigs.k8s.io/k8s-gsm-tools/secret-rotator/client"
	"sigs.k8s.io/k8s-gsm-tools/secret-rotator/config"
	"sigs.k8s.io/k8s-gsm-tools/secret-rotator/rotator"
	"sigs.k8s.io/k8s-gsm-tools/secret-rotator/svckey"
)

type options struct {
	configPath string
	kubeconfig string
}

func (o *options) Validate() error {
	if o.configPath == "" {
		return fmt.Errorf("required flag --config-path was unset")
	}
	return nil
}

func gatherOptions() options {
	o := options{}
	flag.StringVar(&o.configPath, "config-path", "", "Path to config.yaml.")
	flag.StringVar(&o.kubeconfig, "kubeconfig", "", "Path to kubeconfig file.")
	flag.Parse()
	return o
}

func main() {
	klog.InitFlags(nil)

	o := gatherOptions()
	err := o.Validate()
	if err != nil {
		klog.Errorf("Invalid options: %s", err)
	}

	// prepare client
	secretManagerClient, err := client.NewClient(context.Background())
	if err != nil {
		klog.Errorf("Fail to create new Secret Manager client: %s", err)
	}

	// prepare config agent
	configAgent := &config.Agent{}
	runFunc, err := configAgent.WatchConfig(o.configPath)
	if err != nil {
		klog.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go runFunc(ctx)
	defer cancel()

	// prepare provisioners for all supported types of secrets
	provisioners := map[string]rotator.SecretProvisioner{}
	provisioners[svckey.ServiceAccountKeySpec{}.Type()] = &svckey.Provisioner{}

	rotator := &rotator.SecretRotator{
		Client:       secretManagerClient,
		Agent:        configAgent,
		Provisioners: provisioners,
	}

	rotator.RunOnce()
}
