package main

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"time"

	"github.com/coreos/etcd-operator/pkg/analytics"
	"github.com/coreos/etcd-operator/pkg/backup"
	"github.com/coreos/etcd-operator/pkg/backup/env"
	"github.com/coreos/etcd-operator/pkg/backup/s3/s3config"
	k8schaos "github.com/coreos/etcd-operator/pkg/k8s/chaos"
	k8scontroller "github.com/coreos/etcd-operator/pkg/k8s/controller"
	k8sgc "github.com/coreos/etcd-operator/pkg/k8s/garbagecollection"
	"github.com/coreos/etcd-operator/pkg/k8s/k8sutil"
	"github.com/coreos/etcd-operator/pkg/k8s/k8sutil/election"
	"github.com/coreos/etcd-operator/pkg/k8s/k8sutil/election/resourcelock"
	ranchcontroller "github.com/coreos/etcd-operator/pkg/rancher/controller"
	ranchgc "github.com/coreos/etcd-operator/pkg/rancher/garbagecollection"
	"github.com/coreos/etcd-operator/pkg/rancher/ranchutil"
	"github.com/coreos/etcd-operator/pkg/spec"
	"github.com/coreos/etcd-operator/pkg/util/constants"
	"github.com/coreos/etcd-operator/version"

	rancher "github.com/rancher/go-rancher/v2"
	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
	"golang.org/x/time/rate"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api"
	"k8s.io/client-go/pkg/labels"
	"k8s.io/client-go/tools/record"
)

var (
	leaseDuration = 15 * time.Second
	renewDuration = 5 * time.Second
	retryPeriod   = 3 * time.Second
)

func printVersion(c *cli.Context) {
	if c.Command.Name == "" {
		fmt.Printf("etcd-operator Version: %s\n", version.Version)
		fmt.Printf("Git SHA: %s\n", version.GitSHA)
		fmt.Printf("Go Version: %s\n", runtime.Version())
		fmt.Printf("Go OS/Arch: %s/%s\n", runtime.GOOS, runtime.GOARCH)
	} else {
		logrus.Infof("etcd-operator Version: %s", version.Version)
		logrus.Infof("Git SHA: %s", version.GitSHA)
		logrus.Infof("Go Version: %s", runtime.Version())
		logrus.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func main() {
	cli.VersionPrinter = printVersion
	app := &cli.App{
		Name:    "etcd-operator",
		Usage:   "create/configure/manage etcd clusters autonomously",
		Version: version.Version,
		Commands: []cli.Command{
			{
				Name:      "kubernetes",
				ShortName: "k",
				Aliases:   []string{"k8s"},
				Usage:     "operator for Kubernetes",
				Subcommands: []cli.Command{
					{
						Name:        "backup",
						ShortName:   "b",
						Description: "execute the backup policy for a cluster",
						Flags: []cli.Flag{
							cli.StringFlag{
								Name:  "cluster-name",
								Usage: "The etcd cluster name",
							},
							cli.StringFlag{
								Name:  "listen",
								Value: "0.0.0.0:19999",
							},
							cli.StringFlag{
								Name:   "namespace",
								EnvVar: "MY_POD_NAMESPACE",
								Value:  "default",
							},
						},
						Action: kubernetesBackup,
					},
					{
						Name:        "operator",
						ShortName:   "o",
						Description: "run the etcd operator",
						Flags: []cli.Flag{
							cli.BoolTFlag{
								Name:  "analytics",
								Usage: "Send analytical events (Cluster Created/Deleted etc.) to Google Analytics",
							},
							cli.StringFlag{
								Name:  "backup-aws-secret",
								Usage: "The name of the kube secret object that stores the AWS credential file. The file name must be 'credentials'.",
							},
							cli.StringFlag{
								Name:  "backup-aws-config",
								Usage: "The name of the kube configmap object that stores the AWS config file. The file name must be 'config'.",
							},
							cli.StringFlag{
								Name:  "backup-s3-bucket",
								Usage: "The name of the AWS S3 bucket to store backups in.",
							},
							cli.DurationFlag{
								Name:  "gc-interval",
								Usage: "GC interval",
								Value: 10 * time.Minute,
							},
							cli.StringFlag{
								Name:   "name",
								EnvVar: "MY_POD_NAME",
								Hidden: true,
								Usage:  "the k8s pod name this operator is in",
							},
							cli.StringFlag{
								Name:   "namespace",
								EnvVar: "MY_POD_NAMESPACE",
								Hidden: true,
								Usage:  "the k8s namespace this operator is in",
							},
							cli.StringFlag{
								Name:  "pv-provisioner",
								Usage: "persistent volume provisioner type",
								Value: constants.PVProvisionerGCEPD,
							},
							cli.IntFlag{
								Name:  "chaos-level",
								Usage: "DO NOT USE IN PRODUCTION - level of chaos injected into the etcd clusters created by the operator.",
								Value: -1,
							},
						},
						Action: kubernetesOperator,
					},
				},
			},
			{
				Name:      "rancher",
				ShortName: "r",
				Aliases:   []string{"cattle"},
				Usage:     "operator for Rancher",
				Subcommands: []cli.Command{
					{
						Name:        "operator",
						ShortName:   "o",
						Description: "",
						Flags: []cli.Flag{
							cli.BoolTFlag{
								Name:  "analytics",
								Usage: "Send analytical events (Cluster Created/Deleted etc.) to Google Analytics",
							},
							cli.StringFlag{
								Name:  "backup-aws-config",
								Usage: "The name of the kube configmap object that stores the AWS config file. The file name must be 'config'.",
							},
							cli.StringFlag{
								Name:  "backup-aws-secret",
								Usage: "The name of the kube secret object that stores the AWS credential file. The file name must be 'credentials'.",
							},
							cli.StringFlag{
								Name:  "backup-s3-bucket",
								Usage: "The name of the AWS S3 bucket to store backups in.",
							},
							cli.DurationFlag{
								Name:  "gc-interval",
								Usage: "GC interval",
								Value: 10 * time.Minute,
							},
							cli.StringFlag{
								Name:   "namespace",
								EnvVar: "CATTLE_ENVIRONMENT",
								Usage:  "the cattle environment this operator is in",
							},
							cli.StringFlag{
								Name:  "pv-provisioner",
								Usage: "persistent volume provisioner type",
								Value: constants.PVProvisionerGCEPD,
							},
							cli.IntFlag{
								Name:  "chaos-level",
								Usage: "DO NOT USE IN PRODUCTION - level of chaos injected into the etcd clusters created by the operator.",
								Value: -1,
							},
						},
						Action: rancherOperator,
					},
				},
			},
		},
	}
	app.Run(os.Args)
}

func kubernetesBackup(c *cli.Context) error {
	if len(c.String("cluster-name")) == 0 {
		panic("cluster-name not set")
	}

	p := &spec.BackupPolicy{}
	ps := os.Getenv(env.BackupPolicy)
	if err := json.Unmarshal([]byte(ps), p); err != nil {
		logrus.Fatalf("fail to parse backup policy (%s): %v", ps, err)
	}

	kclient := k8sutil.MustNewKubeClient()
	backup.New(kclient, c.String("cluster-name"), c.String("namespace"), *p, c.String("listen")).Run()

	panic("unreachable")
	return nil
}

func operator(c *cli.Context) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch)
	go func() {
		logrus.Infof("received signal: %v", <-ch)
		os.Exit(1)
	}()

	cli.VersionPrinter(c)

	if c.Bool("analytics") {
		analytics.Enable()
	}

	analytics.OperatorStarted()
}

func kubernetesOperator(c *cli.Context) error {
	operator(c)

	// Workaround for watching TPR resource.
	restCfg, err := k8sutil.InClusterConfig()
	if err != nil {
		panic(err)
	}
	k8scontroller.MasterHost = restCfg.Host
	restcli, err := k8sutil.NewTPRClient()
	if err != nil {
		panic(err)
	}
	k8scontroller.KubeHttpCli = restcli.Client

	cfg := newK8sControllerConfig(c)
	if err := cfg.Validate(); err != nil {
		logrus.Fatalf("invalid operator config: %v", err)
	}

	run := func(stop <-chan struct{}) {
		go periodicFullGC(cfg.KubeCli, cfg.Namespace, c.Duration("gc-interval"))

		startChaos(context.Background(), cfg.KubeCli, cfg.Namespace, c.Int("chaos-level"))

		for {
			c := k8scontroller.New(cfg)
			err := c.Run()
			switch err {
			case k8scontroller.ErrVersionOutdated:
			default:
				logrus.Fatalf("controller Run() ended with failure: %v", err)
			}
		}
	}

	electLeader(run, c.String("namespace"))
	panic("unreachable")
	return nil
}

func rancherOperator(c *cli.Context) error {
	operator(c)

	run := func() {
		client := ranchutil.MustNewRancherClient()

		go rancherPeriodicFullGC(client, c.Duration("gc-interval"))

		for {
			c := ranchcontroller.New(client)
			if err := c.Run(); err != nil {
				logrus.Fatalf("controller Run() ended with failure: %v", err)
			}
			// TODO remove
			time.Sleep(5 * time.Second)
		}
	}

	// TODO leader election
	run()
	panic("unreachable")
	return nil
}

func electLeader(run func(stop <-chan struct{}), namespace string) {
	id, err := os.Hostname()
	if err != nil {
		logrus.Fatalf("failed to get hostname: %v", err)
	}

	// TODO: replace this to client-go once leader election package is imported
	//       https://github.com/kubernetes/client-go/issues/28
	rl := &resourcelock.EndpointsLock{
		EndpointsMeta: api.ObjectMeta{
			Namespace: namespace,
			Name:      "etcd-operator",
		},
		Client: k8sutil.MustNewKubeClient(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity:      id,
			EventRecorder: &record.FakeRecorder{},
		},
	}
	election.RunOrDie(election.LeaderElectionConfig{
		Lock:          rl,
		LeaseDuration: leaseDuration,
		RenewDeadline: renewDuration,
		RetryPeriod:   retryPeriod,
		Callbacks: election.LeaderCallbacks{
			OnStartedLeading: run,
			OnStoppedLeading: func() {
				logrus.Fatalf("leader election lost")
			},
		},
	})
}

func newK8sControllerConfig(c *cli.Context) k8scontroller.Config {
	kubecli := k8sutil.MustNewKubeClient()

	pod, err := k8sutil.GetPodByName(kubecli, c.String("namespace"), c.String("name"))
	if err != nil {
		logrus.Fatalf("failed to get my pod: %v", err)
	}
	if pod.Spec.ServiceAccountName == "" {
		logrus.Fatalf("my pod has no service account!")
	}

	return k8scontroller.Config{
		Namespace:      c.String("namespace"),
		ServiceAccount: pod.Spec.ServiceAccountName,
		PVProvisioner:  c.String("pv-provisioner"),
		S3Context: s3config.S3Context{
			AWSSecret: c.String("backup-aws-secret"),
			AWSConfig: c.String("backup-aws-config"),
			S3Bucket:  c.String("backup-s3-bucket"),
		},
		KubeCli: kubecli,
	}
}

func periodicFullGC(kubecli kubernetes.Interface, ns string, d time.Duration) {
	gc := k8sgc.New(kubecli, ns)
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

func startChaos(ctx context.Context, kubecli kubernetes.Interface, ns string, chaosLevel int) {
	m := k8schaos.NewMonkeys(kubecli)
	ls := labels.SelectorFromSet(map[string]string{"app": "etcd"})

	switch chaosLevel {
	case 1:
		logrus.Info("chaos level = 1: randomly kill one etcd pod every 30 seconds at 50%")
		c := &k8schaos.CrashConfig{
			Namespace: ns,
			Selector:  ls,

			KillRate:        rate.Every(30 * time.Second),
			KillProbability: 0.5,
			KillMax:         1,
		}
		go func() {
			time.Sleep(60 * time.Second) // don't start until quorum up
			m.CrushPods(ctx, c)
		}()

	case 2:
		logrus.Info("chaos level = 2: randomly kill at most five etcd pods every 30 seconds at 50%")
		c := &k8schaos.CrashConfig{
			Namespace: ns,
			Selector:  ls,

			KillRate:        rate.Every(30 * time.Second),
			KillProbability: 0.5,
			KillMax:         5,
		}

		go m.CrushPods(ctx, c)

	default:
	}
}

func rancherPeriodicFullGC(client *rancher.RancherClient, d time.Duration) {
	gc := ranchgc.New(client)
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
