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

// This package is a demonstration for secret sync controller.
// A sync controller is configured as specified in fixtureConfig.
// The source secret changes its value every switchPeriod,
// and the destination secret is expected to be synced to the new
// secret value after a short delay.
// Text reports as well as plots for the secret timelines are presented.

import (
	"context"
	"flag"
	"fmt"
	"k8s.io/klog"
	"path/filepath"
	"regexp"
	rotclient "sigs.k8s.io/k8s-gsm-tools/secret-rotator/client"
	syncclient "sigs.k8s.io/k8s-gsm-tools/secret-sync-controller/client"
	syncconfig "sigs.k8s.io/k8s-gsm-tools/secret-sync-controller/config"
	"sigs.k8s.io/k8s-gsm-tools/secret-sync-controller/tests"
	"strconv"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"image/color"
)

type options struct {
	outputPath     string
	syncConfigPath string
	kubeconfig     string
	resyncPeriod   int64
	pollPeriod     int64
	duration       int64
	gsmProject     string
}

func (o *options) Validate() error {
	if o.syncConfigPath == "" {
		return fmt.Errorf("required flag --sync-config was unset")
	}
	return nil
}

func gatherOptions() options {
	o := options{}
	flag.StringVar(&o.outputPath, "output-path", ".", "Desired output path for the result plots.")
	flag.StringVar(&o.syncConfigPath, "config-path", "", "Path to sync-config.yaml.")
	flag.StringVar(&o.gsmProject, "gsm-project", "project-1", "Secret Manager project for demo.")
	flag.StringVar(&o.kubeconfig, "kubeconfig", "", "Path to kubeconfig file.")
	flag.Int64Var(&o.resyncPeriod, "resync-period", 1000, "Resync period in milliseconds.")
	flag.Int64Var(&o.pollPeriod, "poll-period", 500, "Polling period in milliseconds.")
	flag.Int64Var(&o.duration, "duration", 150000, "Logging duration in milliseconds.")
	flag.Parse()
	return o
}

func main() {
	klog.InitFlags(nil)
	flag.Set("v", "1")

	o := gatherOptions()
	err := o.Validate()
	if err != nil {
		klog.Errorf("Invalid options: %s", err)
	}

	// prepare clients
	k8sClientset, err := syncclient.NewK8sClientset(o.kubeconfig)
	if err != nil {
		klog.Errorf("Fail to create new kubernetes client: %s", err)
	}
	secretManagerClient, err := syncclient.NewSecretManagerClient(context.Background())
	if err != nil {
		klog.Errorf("Fail to create new Secret Manager client: %s", err)
	}

	rotationClient, err := rotclient.NewClient(context.Background())
	if err != nil {
		klog.Errorf("Fail to create new rotation client: %s", err)
	}

	testClient := &tests.E2eTestClient{
		&syncclient.Client{
			K8sClientset:        *k8sClientset,
			SecretManagerClient: *secretManagerClient,
		},
	}

	// prepare config agent for secret sync controller
	syncConfigAgent := &syncconfig.Agent{}
	syncUpdateFunc, err := syncConfigAgent.WatchConfig(o.syncConfigPath)
	if err != nil {
		klog.Fatal(err)
	}

	syncCtx, syncCancel := context.WithCancel(context.Background())
	go syncUpdateFunc(syncCtx)
	defer syncCancel()

	logger := &Logger{
		SyncClient: testClient,
		RotClient:  rotationClient,
		Agent:      syncConfigAgent,
		PollPeriod: time.Duration(o.resyncPeriod) * time.Millisecond,
		LogData:    map[string]*logData{},
	}

	// start controller and logger
	stopLogger := make(chan struct{})
	go logger.Start(stopLogger)

	time.Sleep(time.Duration(o.duration) * time.Millisecond)
	stopLogger <- struct{}{}

	logger.ShowResults(o.outputPath)
}

type Logger struct {
	SyncClient syncclient.Interface
	RotClient  *rotclient.Client
	Agent      *syncconfig.Agent
	PollPeriod time.Duration
	LogData    map[string]*logData
}

type logData struct {
	GSMSecretLog []string
	K8sSecretLog []string
	Time         []float64
	States       []string
}

func (l *Logger) Start(stopChan <-chan struct{}) error {
	logChan := make(chan struct{})

	startTime := time.Now()

	// controller period for logging
	go func() {
		for {
			logChan <- struct{}{}
			time.Sleep(l.PollPeriod)
		}
	}()

	for {
		select {
		case <-stopChan:
			klog.V(2).Info("Stop signal received. Quitting syncLogger...")
			return nil
		case <-logChan:
			for _, spec := range l.Agent.Config().Specs {

				d, ok := l.LogData[spec.String()]
				if !ok {
					d = &logData{}
					l.LogData[spec.String()] = d
				}

				// get secret values
				srcData, err := l.SyncClient.GetSecretManagerSecretValue(spec.Source.Project, spec.Source.Secret)
				if err != nil {
					klog.Errorf("Secret log failed for %s: %s", spec, err)
				}

				destData, err := l.SyncClient.GetKubernetesSecretValue(spec.Destination.Namespace, spec.Destination.Secret, spec.Destination.Key)
				if err != nil {
					klog.Errorf("Secret log failed for %s: %s", spec, err)
				}

				labels, err := l.RotClient.GetSecretLabels(spec.Source.Project, spec.Source.Secret)
				if err != nil {
					return err
				}

				active := []string{}

				for key, _ := range labels {
					// the key of a label for [version: data] pair should be in the format of "v%d"
					matched, err := regexp.Match(`^v[0-9]+$`, []byte(key))
					if err != nil {
						return err
					}

					if matched {
						active = append(active, key)
					}
				}

				// append to log
				d.Append(time.Now().Sub(startTime), srcData, destData, active)
			}
		}
	}
}

func (d *logData) Append(t time.Duration, gsm []byte, k8s []byte, active []string) {
	d.Time = append(d.Time, float64(t.Milliseconds())/1000)
	d.GSMSecretLog = append(d.GSMSecretLog, string(gsm))
	d.K8sSecretLog = append(d.K8sSecretLog, string(k8s))

	if i := len(d.Time) - 1; i == 0 {
		klog.Infof("\tK8s secret value intial value: '%s'\n", d.K8sSecretLog[i])
		klog.Infof("\tGSM secret value intial value: '%s'\n", d.GSMSecretLog[i])
	} else {
		if d.GSMSecretLog[i] != d.GSMSecretLog[i-1] {
			klog.Infof("\tGSM secret value updated from '%s' to '%s'\n", d.GSMSecretLog[i-1], d.GSMSecretLog[i])
		}
		if d.K8sSecretLog[i] != d.K8sSecretLog[i-1] {
			klog.Infof("\tK8s secret value updated from '%s' to '%s'\n", d.K8sSecretLog[i-1], d.K8sSecretLog[i])
		}
	}

	if len(d.States) != len(active) {
		d.States = active

		msg := ""
		for _, ver := range active {
			msg += ver + " "
		}

		klog.Infof("\tCurrent active versions: %s", msg)
	}

}

// Plot plots the timeline of secret values of both source and destination
func (d *logData) Plot(name string) {

	p, err := plot.New()
	if err != nil {
		klog.Error(err)
	}

	p.Title.Text = "Secret Sync Timelime"
	p.X.Label.Text = "time (sec.)"
	p.Y.Label.Text = "secret value"

	yMax := 0.0
	n := len(d.Time)
	ptsGSM := make(plotter.XYs, n)
	ptsK8s := make(plotter.XYs, n)
	for i := 0; i < n; i++ {
		ptsGSM[i].X = d.Time[i]
		val, _ := strconv.ParseFloat(d.GSMSecretLog[i], 64)
		ptsGSM[i].Y = val

		if val > yMax {
			yMax = val
		}

		ptsK8s[i].X = d.Time[i]
		val, _ = strconv.ParseFloat(d.K8sSecretLog[i], 64)
		ptsK8s[i].Y = val

		if val > yMax {
			yMax = val
		}
	}

	gsm, err := plotter.NewLine(ptsGSM)
	if err != nil {
		klog.Error(err)
	}

	gsm.LineStyle.Width = vg.Points(1)
	gsm.LineStyle.Color = color.RGBA{R: 255, A: 255}

	k8s, err := plotter.NewLine(ptsK8s)
	if err != nil {
		klog.Error(err)
	}

	k8s.LineStyle.Width = vg.Points(1)
	k8s.LineStyle.Dashes = []vg.Length{vg.Points(5), vg.Points(5)}
	k8s.LineStyle.Color = color.RGBA{B: 255, A: 255}

	p.Add(gsm, k8s)
	p.Legend.Add("Secret Manager secret", gsm)
	p.Legend.Add("Kubernetes secret", k8s)

	// add margins for better visualization
	p.X.Min = -1
	p.X.Max = d.Time[n-1] + 1
	p.Y.Min = -1
	p.Y.Max = yMax

	if err := p.Save(4*vg.Inch, 4*vg.Inch, name); err != nil {
		klog.Error(err)
	}

}

// ShowResults shows the timelines in text form for all secret sync pairs.
// It also outputs a plot "timeline_i" for the ith pair under outputPath.
func (l *Logger) ShowResults(outputPath string) {
	for i, spec := range l.Agent.Config().Specs {
		name := fmt.Sprintf("timeline_%d.png", i)
		name = filepath.Join(outputPath, name)
		d := l.LogData[spec.String()]
		d.Plot(name)
	}
}
