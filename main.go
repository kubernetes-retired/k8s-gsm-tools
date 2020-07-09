package main

import (
	"b01901143.git/secret-sync-controller/client"
	"b01901143.git/secret-sync-controller/config"
	"b01901143.git/secret-sync-controller/controller"
	"context"
	"errors"
	"flag"
	"fmt"
	"gopkg.in/yaml.v2"
	"io/ioutil"
	"k8s.io/klog"
	"os"
	"time"
)

type options struct {
	configPath   string
	testSetup    string
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
	flag.StringVar(&o.testSetup, "test-setup", "", "Path to test-setup.yaml.")
	flag.BoolVar(&o.runOnce, "run-once", false, "Sync once instead of continuous loop.")
	flag.Int64Var(&o.resyncPeriod, "period", 1000, "Resync period in milliseconds.")
	flag.Parse()
	return o
}

func main() {
	o := gatherOptions()

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
	secretSyncConfig, err := LoadConfig(o.configPath)
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

// LoadConfig loads from a yaml file and returns the structure
func LoadConfig(conf string) (*config.SecretSyncConfig, error) {
	stat, err := os.Stat(conf)
	if err != nil {
		return nil, err
	}

	if stat.IsDir() {
		return nil, fmt.Errorf("config cannot be a dir - %s", conf)
	}

	yamlFile, err := ioutil.ReadFile(conf)
	if err != nil {
		return nil, fmt.Errorf("Error reading %s: %s\n", conf, err)
	}

	syncConfig := config.SecretSyncConfig{}
	err = yaml.Unmarshal(yamlFile, &syncConfig)
	if err != nil {
		return nil, fmt.Errorf("Error unmarshalling %s: %s\n", conf, err)
	}

	return &syncConfig, nil
}
