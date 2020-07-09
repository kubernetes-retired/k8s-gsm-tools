package controller

import (
	"b01901143.git/secret-sync-controller/client"
	"b01901143.git/secret-sync-controller/config"
	"bytes"
	"k8s.io/klog"
	"time"
)

type SecretSyncController struct {
	Client       client.Interface
	Config       *config.SecretSyncConfig
	RunOnce      bool
	ResyncPeriod time.Duration
	// TODO: in-struct looger?
}

func (c *SecretSyncController) Start(stopChan <-chan struct{}) error {
	runChan := make(chan struct{})

	go func() {
		for {
			runChan <- struct{}{}
			time.Sleep(c.ResyncPeriod)
		}
	}()

	for {
		select {
		case <-stopChan:
			klog.Info("Stop signal received. Quitting...")
			return nil
		case <-runChan:
			c.SyncAll()
			if c.RunOnce {
				return nil
			}
		}
	}
}

// SyncAll sychronizes all secret pairs specified in Config.Specs.
// Pops error message for any secret pair that it failed to sync or access.
func (c *SecretSyncController) SyncAll() {
	for _, spec := range c.Config.Specs {
		updated, err := c.Sync(spec)
		if err != nil {
			klog.Errorf("Secret sync failed for %s: %s", spec, err)
		}
		if updated {
			klog.Infof("Secret %s synced from %s", spec.Destination, spec.Source)
		}
	}
}

// Sync sychronizes the secret value from spec.Source to spec.Destination.
// Returns true if the secret value in spec.Destination is updated,
// otherwise returns false, meaning that the secret value in spec.Destination remains unchanged.
func (c *SecretSyncController) Sync(spec config.SecretSyncSpec) (bool, error) {
	// get source secret
	srcData, err := c.Client.GetSecretManagerSecretValue(spec.Source.Project, spec.Source.Secret)
	if err != nil {
		return false, err
	}

	// get destination secret
	destData, err := c.Client.GetKubernetesSecretValue(spec.Destination.Namespace, spec.Destination.Secret, spec.Destination.Key)
	if err != nil {
		return false, err
	}

	// update destination secret
	if bytes.Equal(srcData, destData) {
		return false, nil
	}
	// update destination secret value
	// inserts a key-value pair if spec.Destination does not exist yet
	err = c.Client.UpsertKubernetesSecret(spec.Destination.Namespace, spec.Destination.Secret, spec.Destination.Key, srcData)
	if err != nil {
		return false, err
	}

	return true, nil
}
