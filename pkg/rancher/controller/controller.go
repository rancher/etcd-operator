package controller

import (
	"errors"
	"fmt"
	"sync"
	"time"

	"github.com/coreos/etcd-operator/pkg/analytics"
	"github.com/coreos/etcd-operator/pkg/backup/s3/s3config"
	"github.com/coreos/etcd-operator/pkg/common"
	"github.com/coreos/etcd-operator/pkg/rancher/cluster"
	"github.com/coreos/etcd-operator/pkg/rancher/ranchutil"
	"github.com/coreos/etcd-operator/pkg/spec"
	"github.com/coreos/etcd-operator/pkg/util/constants"

	log "github.com/Sirupsen/logrus"
	kwatch "k8s.io/client-go/pkg/watch"
)

var (
	supportedPVProvisioners = map[string]struct{}{
		constants.PVProvisionerGCEPD:  {},
		constants.PVProvisionerAWSEBS: {},
		constants.PVProvisionerNone:   {},
	}

	initRetryWaitTime = 30 * time.Second

	// Workaround for watching TPR resource.
	// client-go has encoding issue and we want something more predictable.
	// KubeHttpCli *http.Client
	MasterHost string
)

const (
	//FIXME make this much higher by default (60s) and configurable
	watchPeriod = 10 * time.Second
)

type Event struct {
	Type   kwatch.EventType
	Object spec.Cluster
}

type Config struct {
	Namespace     string
	PVProvisioner string
	s3config.S3Context
	Client *ranchutil.ContextAwareClient
}

func (c *Config) Validate() error {
	if _, ok := supportedPVProvisioners[c.PVProvisioner]; !ok {
		return fmt.Errorf(
			"persistent volume provisioner %s is not supported: options = %v",
			c.PVProvisioner, supportedPVProvisioners,
		)
	}
	allEmpty := len(c.S3Context.AWSConfig) == 0 && len(c.S3Context.AWSSecret) == 0 && len(c.S3Context.S3Bucket) == 0
	allSet := len(c.S3Context.AWSConfig) != 0 && len(c.S3Context.AWSSecret) != 0 && len(c.S3Context.S3Bucket) != 0
	if !(allEmpty || allSet) {
		return errors.New("AWS/S3 related configs should be all set or all empty")
	}
	return nil
}

type Controller struct {
	logger *log.Entry
	config Config

	clusters  map[string]*cluster.Cluster
	stopChMap map[string]chan struct{}

	waitCluster sync.WaitGroup
}

func New(c Config) Controller {
	return Controller{
		logger:    log.WithField("pkg", "controller"),
		config:    c,
		clusters:  make(map[string]*cluster.Cluster),
		stopChMap: map[string]chan struct{}{},
	}
}

func (c Controller) Run() error {
	defer func() {
		for _, stopC := range c.stopChMap {
			close(stopC)
		}
		c.waitCluster.Wait()
	}()

	for {
		err := c.initResource()
		if err == nil {
			break
		}
		c.logger.Errorf("initialization failed: %v", err)
		c.logger.Infof("retry in %v...", initRetryWaitTime)
		time.Sleep(initRetryWaitTime)
		// todo: add max retry?
	}

	eventCh, errCh := c.watch()

	go func() {
		pt := newPanicTimer(time.Minute, "unexpected long blocking (> 1 Minute) when handling cluster event")

		for event := range eventCh {
			pt.start()
			if err := c.handleClusterEvent(event); err != nil {
				c.logger.Warningf("fail to handle event: %v", err)
			}
			pt.stop()
		}
	}()

	return <-errCh
}

func (c *Controller) initResource() error {
	c.logger.Info("finding existing clusters...")
	clusters, err := c.findAllClusters()
	if err != nil {
		return err
	}
	for _, clus := range clusters {
		stopC := make(chan struct{})
		nc := cluster.New(c.makeClusterConfig(), &clus, stopC, &c.waitCluster)
		c.stopChMap[clus.Metadata.Name] = stopC
		c.clusters[clus.Metadata.Name] = nc
	}
	return nil
}

func (c *Controller) watch() (<-chan *Event, <-chan error) {
	eventCh := make(chan *Event)
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)
		c.detect(eventCh, errCh)
		for _ = range time.NewTicker(watchPeriod).C {
			c.detect(eventCh, errCh)
		}
	}()

	return eventCh, errCh
}

// without versioning of individual resource in rancher, it is necessary to
// periodically poll for clusters and compare them to in-memory clusters in
// order to detect events.
func (c *Controller) detect(eventCh chan<- *Event, errCh chan<- error) {
	newClusters, err := c.findAllClusters()
	if err != nil {
		errCh <- err
		return
	}
	for _, newCluster := range newClusters {
		oldCluster, ok := c.clusters[newCluster.Metadata.Name]
		if !ok {
			eventCh <- &Event{Type: kwatch.Added, Object: newCluster}
		} else if !oldCluster.Get().Equals(newCluster) {
			eventCh <- &Event{Type: kwatch.Modified, Object: newCluster}
		}
	}
	for _, oldCluster := range c.clusters {
		if _, ok := newClusters[oldCluster.Get().Metadata.Name]; !ok {
			eventCh <- &Event{Type: kwatch.Deleted, Object: *oldCluster.Get()}
		}
	}
}

func (c *Controller) handleClusterEvent(event *Event) error {
	c.logger.Debugf("Received event: %+v", event)
	clus := event.Object

	if clus.Status.IsFailed() {
		if event.Type == kwatch.Deleted {
			delete(c.clusters, clus.Metadata.Name)
			return nil
		}
		return fmt.Errorf("ignore failed cluster (%s). Please delete its TPR", clus.Metadata.Name)
	}

	clus.Spec.Cleanup()

	switch event.Type {
	case kwatch.Added:
		stopC := make(chan struct{})
		nc := cluster.New(c.makeClusterConfig(), &clus, stopC, &c.waitCluster)

		c.stopChMap[clus.Metadata.Name] = stopC
		c.clusters[clus.Metadata.Name] = nc

		analytics.ClusterCreated()
		common.ClustersCreated.Inc()
		common.ClustersTotal.Inc()

	case kwatch.Modified:
		if _, ok := c.clusters[clus.Metadata.Name]; !ok {
			return fmt.Errorf("unsafe state. cluster was never created but we received event (%s)", event.Type)
		}
		c.clusters[clus.Metadata.Name].Update(&clus)
		common.ClustersModified.Inc()

	case kwatch.Deleted:
		if _, ok := c.clusters[clus.Metadata.Name]; !ok {
			return fmt.Errorf("unsafe state. cluster was never created but we received event (%s)", event.Type)
		}
		c.clusters[clus.Metadata.Name].Delete()
		delete(c.clusters, clus.Metadata.Name)
		analytics.ClusterDeleted()
		common.ClustersDeleted.Inc()
		common.ClustersTotal.Dec()
	}
	return nil
}

// WARNING: calling this function has a side effect of updating
// services to work around a few bugs.
func (c *Controller) findAllClusters() (map[string]spec.Cluster, error) {
	services, err := c.config.Client.ListEtcdServices("")
	if err != nil {
		return nil, err
	}

	clusters := make(map[string]spec.Cluster)
	for _, s := range services {

		// we need to update each service proactively to work around
		// bugs/limitation of rancher ui service creation
		if s.Scale > 0 {
			s2 := s
			s2.SelectorContainer = fmt.Sprintf("app=etcd,cluster=%s", s.Id)
			s2.Scale = 0
			s2.StartOnCreate = false
			// we have to adjust the context here from global -> environment to make changes
			ranchutil.SetResourceContext(&s.Resource, s.AccountId)
			if _, err := c.config.Client.Env(s.AccountId).Service.Update(&s, &s2); err != nil {
				log.Warnf("couldn't update service: %s", err)
			}
		}

		cluster := ranchutil.ClusterFromService(s)
		//c.logger.Debugf("cluster: %+v", cluster)
		clusters[cluster.Metadata.Name] = cluster
	}
	//c.logger.Debugf("clusters: %+v", clusters)
	return clusters, nil
}

func (c *Controller) makeClusterConfig() cluster.Config {
	return cluster.Config{
		PVProvisioner: c.config.PVProvisioner,
		S3Context:     c.config.S3Context,

		Client: c.config.Client,
	}
}

// panicTimer panics when it reaches the given duration.
type panicTimer struct {
	d   time.Duration
	msg string
	t   *time.Timer
}

func newPanicTimer(d time.Duration, msg string) *panicTimer {
	return &panicTimer{
		d:   d,
		msg: msg,
	}
}

func (pt *panicTimer) start() {
	pt.t = time.AfterFunc(pt.d, func() {
		panic(pt.msg)
	})
}

// stop stops the timer and resets the elapsed duration.
func (pt *panicTimer) stop() {
	if pt.t != nil {
		pt.t.Stop()
		pt.t = nil
	}
}
