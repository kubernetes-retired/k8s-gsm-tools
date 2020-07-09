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
	"errors"
	"flag"
	"github.com/b01901143/secret-sync-controller/pkg/client"
	"github.com/b01901143/secret-sync-controller/pkg/config"
	"github.com/b01901143/secret-sync-controller/pkg/controller"
	"k8s.io/klog"
	"time"
)

type options struct {
	configPath   string
	runOnce      bool
	resyncPeriod int64
}

func (o *options) Validate() error {
	if o.configPath == "" {
		return errors.New("required flag --config-path was unset")
	}
	return nil
}

func gatherOptions() options {
	o := options{}
	flag.StringVar(&o.configPath, "config-path", "", "Path to config.yaml.")
	flag.BoolVar(&o.runOnce, "run-once", false, "Sync once instead of continuous loop.")
	flag.Int64Var(&o.resyncPeriod, "period", 1000, "Resync period in milliseconds.")
	flag.Parse()
	return o
}

func main() {
	o := gatherOptions()

	err := o.Validate()
	if err != nil {
		klog.Errorf("Invalid options: %s", err)
	}

	// prepare clients
	k8sClientset, err := client.NewK8sClientset()
	if err != nil {
		klog.Errorf("Fail to create new kubernetes client: %s", err)
	}
	secretManagerClient, err := client.NewSecretManagerClient(context.Background())
	if err != nil {
		klog.Errorf("Fail to create new Secret Manager client: %s", err)
	}
	clientInterface := &client.Client{
		K8sClientset:        *k8sClientset,
		SecretManagerClient: *secretManagerClient,
	}

	// prepare config
	secretSyncConfig := &config.SecretSyncConfig{}
	err = secretSyncConfig.LoadFrom(o.configPath)
	if err != nil {
		klog.Errorf("Fail to load config: %s", err)
	}

	err = secretSyncConfig.Validate()
	if err != nil {
		klog.Errorf("Fail to validate: %s", err)
	}

	controller := &controller.SecretSyncController{
		Client:       clientInterface,
		Config:       secretSyncConfig,
		RunOnce:      o.runOnce,
		ResyncPeriod: time.Duration(o.resyncPeriod) * time.Millisecond,
	}

	var stopChan <-chan struct{}
	controller.Start(stopChan)

}
