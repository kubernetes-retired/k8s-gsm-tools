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
	"sigs.k8s.io/k8s-gsm-tools/experiment/svc-consumer/keys"
	"sigs.k8s.io/k8s-gsm-tools/experiment/svc-consumer/logger"
	"time"
)

type options struct {
	mountPath  string
	outputPath string
	period     int64
	gsmProject string
}

func (o *options) Validate() error {
	if o.mountPath == "" {
		return fmt.Errorf("required flag --mount-path was unset")
	}
	return nil
}

func gatherOptions() options {
	o := options{}
	flag.StringVar(&o.mountPath, "mount-path", "", "Path to mounted svc key.")
	flag.StringVar(&o.outputPath, "output-path", "consumer_keys", "Output path for svc keys.")
	flag.StringVar(&o.gsmProject, "gsm-project", "", "Secret Manager project.")
	flag.Int64Var(&o.period, "period", 1000, "Period in milliseconds.")
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

	// prepare keys agent
	keysAgent := &keys.Agent{
		Dir: o.outputPath,
	}
	runFunc, err := keysAgent.WatchMounted(o.mountPath)
	if err != nil {
		klog.Fatal(err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	go runFunc(ctx)
	defer cancel()

	logger := logger.Logger{
		Agent:   keysAgent,
		Period:  time.Duration(o.period) * time.Millisecond,
		Project: o.gsmProject,
	}

	stopChan := make(chan struct{})
	logger.Start(stopChan)
}
