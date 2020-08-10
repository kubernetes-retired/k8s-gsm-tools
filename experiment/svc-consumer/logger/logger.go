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

package logger

import (
	"context"
	"google.golang.org/api/option"
	"k8s.io/klog"
	"sigs.k8s.io/k8s-gsm-tools/experiment/svc-consumer/keys"
	"time"

	secretmanager "cloud.google.com/go/secretmanager/apiv1"
	secretmanagerpb "google.golang.org/genproto/googleapis/cloud/secretmanager/v1"
)

type Logger struct {
	Agent   *keys.Agent
	Period  time.Duration
	Project string
}

// Start starts the logger in continuous mode.
// stops when stop sinal is received from stopChan.
func (l *Logger) Start(stopChan <-chan struct{}) error {
	runChan := make(chan struct{})

	go func() {
		for {
			runChan <- struct{}{}
			time.Sleep(l.Period)
		}
	}()

	for {
		select {
		case <-stopChan:
			klog.V(2).Info("Stop signal received. Quitting...")
			return nil
		case <-runChan:
			l.RunOnce()
		}
	}
}

// RunOnce checks all rotated service account keys in Agent.GetKeys(),
// pings all versions of the mounted service account key, to check the validity of each.
func (l *Logger) RunOnce() {
	for _, keyPath := range l.Agent.GetKeys() {
		ctx := context.TODO()
		client, _ := secretmanager.NewClient(ctx, option.WithCredentialsFile(keyPath))

		name := "projects/" + l.Project

		req := &secretmanagerpb.ListSecretsRequest{
			Parent: name,
		}
		it := client.ListSecrets(ctx, req)
		_, err := it.Next()
		if err != nil {
			klog.Infof("[invalid] %s", keyPath)
		} else {
			klog.Infof("[valid] %s", keyPath)
		}
	}
	klog.Infof("============================")
}
