package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	"github.com/coreos/etcd-operator/pkg/backup/env"
	"github.com/coreos/etcd-operator/pkg/k8s"
	"github.com/coreos/etcd-operator/pkg/k8s/chaos"
	"github.com/coreos/etcd-operator/pkg/k8s/k8sutil"
	"github.com/coreos/etcd-operator/pkg/k8s/k8sutil/election"
	"github.com/coreos/etcd-operator/pkg/k8s/k8sutil/election/resourcelock"
	"github.com/coreos/etcd-operator/pkg/rancher"
	"github.com/coreos/etcd-operator/pkg/util/constants"
	"github.com/coreos/etcd-operator/version"

	log "github.com/Sirupsen/logrus"
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
		log.Infof("etcd-operator Version: %s", version.Version)
		log.Infof("Git SHA: %s", version.GitSHA)
		log.Infof("Go Version: %s", runtime.Version())
		log.Infof("Go OS/Arch: %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}

func main() {
	cli.VersionPrinter = printVersion

	operatorFlags := []cli.Flag{
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
			Name:  "pv-provisioner",
			Usage: "persistent volume provisioner type",
			Value: constants.PVProvisionerGCEPD,
		},
		cli.IntFlag{
			Name:  "chaos-level",
			Usage: "DO NOT USE IN PRODUCTION - level of chaos injected into the etcd clusters created by the operator.",
			Value: -1,
		},
	}
	backupFlags := []cli.Flag{
		cli.StringFlag{
			Name:  "cluster-name",
			Usage: "The etcd cluster name",
		},
		cli.StringFlag{
			Name:  "listen",
			Value: "0.0.0.0:19999",
		},
		cli.StringFlag{
			Name:   "backup-policy",
			EnvVar: env.BackupPolicy,
		},
	}

	app := &cli.App{
		Name:    "etcd-operator",
		Usage:   "create/configure/manage etcd clusters autonomously",
		Version: version.Version,
		Flags: []cli.Flag{
			cli.BoolFlag{
				Name:  "color",
				Usage: "Force colored logging",
			},
			cli.BoolFlag{
				Name:  "debug",
				Usage: "Enable debug logging",
			},
		},
		Before: func(c *cli.Context) error {
			cli.VersionPrinter(c)
			if c.Bool("debug") {
				log.SetLevel(log.DebugLevel)
			}
			if c.Bool("color") {
				log.SetFormatter(&log.TextFormatter{ForceColors: true})
			}
			return nil
		},
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
						Description: "execute the backup manager for a cluster",
						Flags: append(backupFlags, []cli.Flag{
							cli.StringFlag{
								Name:   "namespace",
								EnvVar: "MY_POD_NAMESPACE",
								Value:  "default",
							},
						}...),
						Action: kubernetesBackup,
					},
					{
						Name:        "operator",
						ShortName:   "o",
						Description: "run the etcd operator for kubernetes",
						Flags: append(operatorFlags, []cli.Flag{
							cli.StringFlag{
								Name:   "name",
								EnvVar: "MY_POD_NAME",
								Hidden: true,
								Usage:  "the k8s pod name this operator resides in",
							},
							cli.StringFlag{
								Name:   "namespace",
								EnvVar: "MY_POD_NAMESPACE",
								Hidden: true,
								Usage:  "the k8s namespace this operator resides in",
							},
						}...),
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
						Name:        "backup",
						ShortName:   "b",
						Description: "execute the backup manager for a cluster",
						Flags:       backupFlags,
						Action:      rancherBackup,
					},
					{
						Name:        "operator",
						ShortName:   "o",
						Description: "run the etcd operator for rancher",
						Flags:       operatorFlags,
						Action:      rancherOperator,
					},
				},
			},
		},
	}
	app.Run(os.Args)
}

func kubernetesBackup(c *cli.Context) error {
	k8s.NewBackupManager(c).Run()
	return nil
}

func rancherBackup(c *cli.Context) error {
	rancher.NewBackupManager(c).Run()
	return nil
}

func kubernetesOperator(c *cli.Context) error {
	op := k8s.NewOperator(c)
	electLeader(op.Run, c.String("namespace"))
	return nil
}

func rancherOperator(c *cli.Context) error {
	// TODO leader election
	rancher.NewOperator(c).Run(nil)
	return nil
}

func electLeader(run func(stop <-chan struct{}), namespace string) {
	id, err := os.Hostname()
	if err != nil {
		log.Fatalf("failed to get hostname: %v", err)
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
				log.Fatalf("leader election lost")
			},
		},
	})
}

func startChaos(ctx context.Context, kubecli kubernetes.Interface, ns string, chaosLevel int) {
	m := chaos.NewMonkeys(kubecli)
	ls := labels.SelectorFromSet(map[string]string{"app": "etcd"})

	switch chaosLevel {
	case 1:
		log.Info("chaos level = 1: randomly kill one etcd pod every 30 seconds at 50%")
		c := &chaos.CrashConfig{
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
		log.Info("chaos level = 2: randomly kill at most five etcd pods every 30 seconds at 50%")
		c := &chaos.CrashConfig{
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
