package rancher

import (
	"github.com/coreos/etcd-operator/pkg/rancher/controller"
	"github.com/coreos/etcd-operator/pkg/rancher/garbagecollection"
	"github.com/coreos/etcd-operator/pkg/rancher/ranchutil"
	"github.com/coreos/etcd-operator/pkg/spec"

	"github.com/urfave/cli"
)

func NewRancherOperator(c *cli.Context) *spec.Operator {
	client := ranchutil.MustNewRancherClient()

	return &spec.Operator{
		Controller: controller.New(client),
		GC:         garbagecollection.New(client),
		GCPeriod:   c.Duration("gc-interval"),
		OptIn:      c.Bool("analytics"),
	}
}
