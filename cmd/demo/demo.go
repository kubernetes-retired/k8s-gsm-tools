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
	"bytes"
	"context"
	"flag"
	"fmt"
	"k8s.io/klog"
	"path/filepath"
	"sigs.k8s.io/k8s-gsm-tools/secret-sync-controller/client"
	"sigs.k8s.io/k8s-gsm-tools/secret-sync-controller/config"
	"sigs.k8s.io/k8s-gsm-tools/secret-sync-controller/controller"
	"sigs.k8s.io/k8s-gsm-tools/tests"
	"strconv"
	"text/template"
	"time"

	"gonum.org/v1/plot"
	"gonum.org/v1/plot/plotter"
	"gonum.org/v1/plot/vg"
	"image/color"
)

type options struct {
	outputPath   string
	configPath   string
	kubeconfig   string
	resyncPeriod int64
	pollPeriod   int64
	switchPeriod int64
	duration     int64
	gsmProject   string
}

func (o *options) Validate() error {
	if o.configPath == "" {
		return fmt.Errorf("required flag --config-path was unset")
	}
	return nil
}

func gatherOptions() options {
	o := options{}
	flag.StringVar(&o.outputPath, "output-path", ".", "Desired output path for the result plots.")
	flag.StringVar(&o.configPath, "config-path", "", "Path to config.yaml.")
	flag.StringVar(&o.gsmProject, "gsm-project", "project-1", "Secret Manager project for demo.")
	flag.StringVar(&o.kubeconfig, "kubeconfig", "", "Path to kubeconfig file.")
	flag.Int64Var(&o.resyncPeriod, "resync-period", 1000, "Resync period in milliseconds.")
	flag.Int64Var(&o.pollPeriod, "poll-period", 500, "Polling period in milliseconds.")
	flag.Int64Var(&o.switchPeriod, "switch-period", 30000, "Src secret value switching period in milliseconds.")
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
	k8sClientset, err := client.NewK8sClientset(o.kubeconfig)
	if err != nil {
		klog.Errorf("Fail to create new kubernetes client: %s", err)
	}
	secretManagerClient, err := client.NewSecretManagerClient(context.Background())
	if err != nil {
		klog.Errorf("Fail to create new Secret Manager client: %s", err)
	}

	testClient := &tests.E2eTestClient{
		&client.Client{
			K8sClientset:        *k8sClientset,
			SecretManagerClient: *secretManagerClient,
		},
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

	controller := &controller.SecretSyncController{
		Client:       testClient,
		Agent:        configAgent,
		ResyncPeriod: time.Duration(o.resyncPeriod) * time.Millisecond,
	}

	logger := &SecretSyncLogger{
		Client:       testClient,
		Agent:        configAgent,
		PollPeriod:   time.Duration(o.resyncPeriod) * time.Millisecond,
		SwitchPeriod: time.Duration(o.switchPeriod) * time.Millisecond,
		LogData:      map[string]*logData{},
	}

	//set up demo environment
	var fixtureConfig = `
      secretmanager:
        {{.}}: 
          secret-1: 0
          secret-2: 1
      kubernetes:
        ns-a:
          secret-a:
            key-a: -1
        ns-b:
          secret-b:
            key-b: -1
    `
	// substitute fixture project to o.gsmProject
	temp := template.Must(template.New("config").Parse(fixtureConfig))
	fixtureBuffer := new(bytes.Buffer)
	temp.Execute(fixtureBuffer, o.gsmProject)

	fixture, err := tests.NewFixture(fixtureBuffer.Bytes())
	if err != nil {
		klog.Fatalf("Fail to parse fixture: %s", err)
	}

	err = fixture.Setup(testClient)
	if err != nil {
		klog.Fatalf("Fail to setup fixture: %s", err)
	}

	defer fixture.Teardown(testClient)

	// start controller and logger
	stopLogger := make(chan struct{})
	stopController := make(chan struct{})
	go logger.Start(stopLogger)
	go controller.Start(stopController)

	time.Sleep(time.Duration(o.duration) * time.Millisecond)
	stopLogger <- struct{}{}
	stopController <- struct{}{}

	logger.ShowResults(o.outputPath)
}

type SecretSyncLogger struct {
	Client       client.Interface
	Agent        *config.Agent
	PollPeriod   time.Duration
	SwitchPeriod time.Duration
	LogData      map[string]*logData
}

type logData struct {
	GSMSecretLog []string
	K8sSecretLog []string
	Time         []float64
}

func (l *SecretSyncLogger) Start(stopChan <-chan struct{}) error {
	logChan := make(chan struct{})
	switchChan := make(chan struct{})

	startTime := time.Now()

	// controller period for logging
	go func() {
		for {
			logChan <- struct{}{}
			time.Sleep(l.PollPeriod)
		}
	}()

	// controller period for source secret value switch
	go func() {
		for {
			switchChan <- struct{}{}
			time.Sleep(l.SwitchPeriod)
		}
	}()

	for {
		select {
		case <-stopChan:
			klog.V(2).Info("Stop signal received. Quitting logger...")
			return nil
		case <-logChan:
			for _, spec := range l.Agent.Config().Specs {

				d, ok := l.LogData[spec.String()]
				if !ok {
					d = &logData{}
					l.LogData[spec.String()] = d
				}

				// get secret values
				srcData, err := l.Client.GetSecretManagerSecretValue(spec.Source.Project, spec.Source.Secret)
				if err != nil {
					klog.Errorf("Secret log failed for %s: %s", spec, err)
				}

				destData, err := l.Client.GetKubernetesSecretValue(spec.Destination.Namespace, spec.Destination.Secret, spec.Destination.Key)
				if err != nil {
					klog.Errorf("Secret log failed for %s: %s", spec, err)
				}

				// append to log
				d.Append(time.Now().Sub(startTime), srcData, destData)
			}
		case <-switchChan:
			for _, spec := range l.Agent.Config().Specs {
				prev, _ := l.Client.GetSecretManagerSecretValue(spec.Source.Project, spec.Source.Secret)

				// secret value swaps between 0 and 1 every SwitchPeriod
				next := "0"
				if next == string(prev) {
					next = "1"
				}
				l.Client.UpsertSecretManagerSecret(spec.Source.Project, spec.Source.Secret, []byte(next))
			}
		}
	}
}

func (d *logData) Append(t time.Duration, gsm []byte, k8s []byte) {
	d.Time = append(d.Time, t.Seconds())
	d.GSMSecretLog = append(d.GSMSecretLog, string(gsm))
	d.K8sSecretLog = append(d.K8sSecretLog, string(k8s))
}

func (d *logData) TextReport() string {
	msg := ""
	for i := range d.Time {
		if i == 0 {
			msg += fmt.Sprintf("\t[%v] K8s secret value intial value: '%s'\n", time.Duration(d.Time[i])*time.Second, d.K8sSecretLog[i])
			msg += fmt.Sprintf("\t[%v] GSM secret value intial value: '%s'\n", time.Duration(d.Time[i])*time.Second, d.GSMSecretLog[i])
		} else {
			if d.GSMSecretLog[i] != d.GSMSecretLog[i-1] {
				msg += fmt.Sprintf("\t[%v] GSM secret value updated from '%s' to '%s'\n", time.Duration(d.Time[i])*time.Second, d.GSMSecretLog[i-1], d.GSMSecretLog[i])
			}
			if d.K8sSecretLog[i] != d.K8sSecretLog[i-1] {
				msg += fmt.Sprintf("\t[%v] K8s secret value updated from '%s' to '%s'\n", time.Duration(d.Time[i])*time.Second, d.K8sSecretLog[i-1], d.K8sSecretLog[i])
			}
		}

	}
	return msg
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

	n := len(d.Time)
	ptsGSM := make(plotter.XYs, n)
	ptsK8s := make(plotter.XYs, n)
	for i := 0; i < n; i++ {
		ptsGSM[i].X = d.Time[i]
		val, _ := strconv.ParseFloat(d.GSMSecretLog[i], 64)
		ptsGSM[i].Y = val

		ptsK8s[i].X = d.Time[i]
		val, _ = strconv.ParseFloat(d.K8sSecretLog[i], 64)
		ptsK8s[i].Y = val
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
	p.Y.Max = 2

	if err := p.Save(4*vg.Inch, 4*vg.Inch, name); err != nil {
		klog.Error(err)
	}

}

// ShowResults shows the timelines in text form for all secret sync pairs.
// It also outputs a plot "timeline_i" for the ith pair under outputPath.
func (l *SecretSyncLogger) ShowResults(outputPath string) {
	for i, spec := range l.Agent.Config().Specs {
		name := fmt.Sprintf("timeline_%d.png", i)
		name = filepath.Join(outputPath, name)
		d := l.LogData[spec.String()]
		d.Plot(name)

		klog.V(1).Infof("Results for secretSyncPair[%d]:{\n%s}", i, d.TextReport())
	}
}
