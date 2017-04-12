package controller

import (
	"sync"
	"time"

	"github.com/coreos/etcd-operator/pkg/rancher/ranchutil"
	"github.com/coreos/etcd-operator/pkg/spec"

	log "github.com/Sirupsen/logrus"
	"github.com/coreos/etcd-operator/pkg/k8s/cluster"
)

type Controller struct {
	log    *log.Entry
	client *ranchutil.ContextAwareClient

	clusters map[string]map[string]*cluster.Cluster
	stopChMap  map[string]chan struct{}

	waitCluster sync.WaitGroup
}

func New(client *ranchutil.ContextAwareClient) Controller {
	return Controller{
		log:    log.WithField("pkg", "controller"),
		client: client,

		clusters:   make(map[string]map[string]*cluster.Cluster),
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
	for range t.C {
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
	for _, s := range c.client.ListEtcdServices("") {
		clusters = append(clusters, ranchutil.ClusterFromService(s))
	}
	return clusters
}
