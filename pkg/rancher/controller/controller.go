package controller

import (
	"sync"
	"time"

	"github.com/coreos/etcd-operator/pkg/rancher/ranchutil"
	"github.com/coreos/etcd-operator/pkg/spec"

	log "github.com/Sirupsen/logrus"
	"github.com/coreos/etcd-operator/pkg/k8s/cluster"
	rancher "github.com/rancher/go-rancher/v2"
)

type Controller struct {
	log     *log.Entry
	rclient *rancher.RancherClient

	// TODO: combine the three cluster map.
	clusters map[string]*cluster.Cluster
	// Kubernetes resource version of the clusters
	clusterRVs map[string]string
	stopChMap  map[string]chan struct{}

	waitCluster sync.WaitGroup
}

func New(client *rancher.RancherClient) Controller {
	return Controller{
		log:     log.WithField("pkg", "controller"),
		rclient: client,

		clusters:   make(map[string]*cluster.Cluster),
		clusterRVs: make(map[string]string),
		stopChMap:  map[string]chan struct{}{},
	}
}

func (c Controller) Run() error {
	// stack := ranchutil.NewStack("etcd", "managed by etcd operator")
	// c.CreateStack(env.Id, stack)

	// service := ranchutil.NewEtcdService("etcd", stack.Id)
	// c.CreateService(env.Id, service)

	//container := ranchutil.NewEtcdContainer("etcd", service.Id)
	//c.CreateContainer(env.Id, container)

	c.periodicallyReconcile()
	return nil
}

func (c *Controller) periodicallyReconcile() {
	c.reconcile()
	t := time.NewTicker(8 * time.Second)
	for _ = range t.C {
		c.reconcile()
	}
	panic("unreachable")
}

func (c *Controller) reconcile() {
	c.log.Debugf("begin reconciliation")
	defer c.log.Debugf("end reconciliation")

	for _, cluster := range c.findClusters() {
		c.log.Infof("  reconciling %+v", cluster)
	}
}

func (c *Controller) findClusters() []spec.Cluster {
	var clusters []spec.Cluster
	for _, s := range c.findServices() {
		clusters = append(clusters, ranchutil.ClusterFromService(&s))
	}
	return clusters
}

func (c *Controller) findServices() []rancher.Service {
	services := []rancher.Service{}
	col, err := c.rclient.Service.List(&rancher.ListOpts{})
	if err != nil {
		return services
	}
	c.log.Debugf("found %d services total", len(col.Data))
	for _, s := range col.Data {
		if s.LaunchConfig != nil && s.LaunchConfig.Labels != nil {
			if _, ok := s.LaunchConfig.Labels["io.rancher.operator"]; ok {
				//c.log.Infof("%s has launch config labels %+v", s.Name, s.LaunchConfig.Labels)
				services = append(services, s)
			}
		}
	}
	c.log.Debugf("found %d etcd services", len(services))
	return services
}
