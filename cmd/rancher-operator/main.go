// Copyright 2016 The etcd-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/coreos/etcd-operator/pkg/analytics"
	"github.com/coreos/etcd-operator/pkg/rancher/controller"
	"github.com/coreos/etcd-operator/pkg/rancher/garbagecollection"
	"github.com/coreos/etcd-operator/pkg/util/constants"
	"github.com/coreos/etcd-operator/pkg/util/rancherutil"
	"github.com/coreos/etcd-operator/version"
	rancher "github.com/rancher/go-rancher/v2"

	"github.com/Sirupsen/logrus"
)

var (
	analyticsEnabled bool
	pvProvisioner    string
	environment      string
	awsSecret        string
	awsConfig        string
	s3Bucket         string
	gcInterval       time.Duration

	chaosLevel int

	printVersion bool
)

var (
	leaseDuration = 15 * time.Second
	renewDuration = 5 * time.Second
	retryPeriod   = 3 * time.Second
)

func init() {
	flag.BoolVar(&analyticsEnabled, "analytics", false, "Send analytical event (Cluster Created/Deleted etc.) to Google Analytics")

	// TODO integrate with DVP
	flag.StringVar(&pvProvisioner, "pv-provisioner", constants.PVProvisionerGCEPD, "persistent volume provisioner type")
	flag.StringVar(&awsSecret, "backup-aws-secret", "",
		"The name of the kube secret object that stores the AWS credential file. The file name must be 'credentials'.")
	flag.StringVar(&awsConfig, "backup-aws-config", "",
		"The name of the kube configmap object that stores the AWS config file. The file name must be 'config'.")
	flag.StringVar(&s3Bucket, "backup-s3-bucket", "", "The name of the AWS S3 bucket to store backups in.")
	// chaos level will be removed once we have a formal tool to inject failures.
	flag.IntVar(&chaosLevel, "chaos-level", -1, "DO NOT USE IN PRODUCTION - level of chaos injected into the etcd clusters created by the operator.")
	flag.BoolVar(&printVersion, "version", false, "Show version and quit")
	flag.DurationVar(&gcInterval, "gc-interval", 10*time.Minute, "GC interval")
	flag.Parse()
}

func main() {
	environment = os.Getenv("CATTLE_ENVIRONMENT")
	if len(environment) == 0 {
		logrus.Fatalf("must set env CATTLE_ENVIRONMENT")
	}

	c := make(chan os.Signal, 1)
	signal.Notify(c)
	go func() {
		logrus.Infof("received signal: %v", <-c)
		os.Exit(1)
	}()

	if printVersion {
		fmt.Println("etcd-operator Version:", version.Version)
		fmt.Println("Git SHA:", version.GitSHA)
		fmt.Println("Go Version:", runtime.Version())
		fmt.Printf("Go OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
		os.Exit(0)
	}

	logrus.Infof("etcd-operator Version: %v", version.Version)
	logrus.Infof("Git SHA: %s", version.GitSHA)
	logrus.Infof("Go Version: %s", runtime.Version())
	logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)

	if analyticsEnabled {
		analytics.Enable()
	}

	analytics.OperatorStarted()

	_, err := os.Hostname()
	if err != nil {
		logrus.Fatalf("failed to get hostname: %v", err)
	}

	run()
	panic("unreachable")
}

func run() {
	client := rancherutil.MustNewRancherClient()

	go periodicFullGC(client, gcInterval)

	for {
		c := controller.New(client)
		if err := c.Run(); err != nil {
			logrus.Fatalf("controller Run() ended with failure: %v", err)
		}
		time.Sleep(5 * time.Second)
	}
}

func periodicFullGC(client *rancher.RancherClient, d time.Duration) {
	gc := garbagecollection.New(client, environment)
	timer := time.NewTimer(d)
	defer timer.Stop()
	for {
		<-timer.C
		err := gc.FullyCollect()
		if err != nil {
			logrus.Warningf("failed to cleanup resources: %v", err)
		}
	}
}
