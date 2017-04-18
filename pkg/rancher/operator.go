package rancher

import (
	"github.com/coreos/etcd-operator/pkg/backup/s3/s3config"
	"github.com/coreos/etcd-operator/pkg/rancher/controller"
	"github.com/coreos/etcd-operator/pkg/rancher/garbagecollection"
	"github.com/coreos/etcd-operator/pkg/rancher/ranchutil"
	"github.com/coreos/etcd-operator/pkg/spec"

	"github.com/Sirupsen/logrus"
	"github.com/urfave/cli"
)

func NewOperator(c *cli.Context) *spec.Operator {
	client := ranchutil.NewContextAwareClient()

	cfg := newControllerConfig(c, client)
	if err := cfg.Validate(); err != nil {
		logrus.Fatalf("invalid operator config: %v", err)
	}

	return &spec.Operator{
		Controller: controller.New(cfg),
		GC:         garbagecollection.New(client),
		GCPeriod:   c.Duration("gc-interval"),
		OptIn:      c.Bool("analytics"),
	}
}

func newControllerConfig(c *cli.Context, client *ranchutil.ContextAwareClient) controller.Config {
	return controller.Config{
		Client:        client,
		Namespace:     c.String("namespace"),
		PVProvisioner: c.String("pv-provisioner"),
		S3Context: s3config.S3Context{
			AWSSecret: c.String("backup-aws-secret"),
			AWSConfig: c.String("backup-aws-config"),
			S3Bucket:  c.String("backup-s3-bucket"),
		},
	}
}
