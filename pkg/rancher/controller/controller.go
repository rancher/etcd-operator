package controller

import (
	"reflect"
	"sync"
	"time"

	"github.com/coreos/etcd-operator/pkg/rancher/ranchutil"
	"github.com/coreos/etcd-operator/pkg/spec"

	log "github.com/Sirupsen/logrus"
	kwatch "k8s.io/client-go/pkg/watch"
)

type Event struct {
	Type   kwatch.EventType
	Object *spec.Cluster
}

type Controller struct {
	log    *log.Entry
	client *ranchutil.ContextAwareClient

	clusters  map[string]*spec.Cluster
	stopChMap map[string]chan struct{}

	waitCluster sync.WaitGroup
}

func New(client *ranchutil.ContextAwareClient) Controller {
	return Controller{
		log:    log.WithField("pkg", "controller"),
		client: client,

		clusters:   make(map[string]*spec.Cluster),
		stopChMap:  map[string]chan struct{}{},
	}
}

func (c Controller) Run() error {
	eventCh, errCh := c.watch()

	go func() {
		pt := newPanicTimer(time.Minute, "unexpected long blocking (> 1 Minute) when handling cluster event")

		for event := range eventCh {
			pt.start()
			if err := c.handle(event); err != nil {
				c.log.Warningf("fail to handle event: %v", err)
			}
			pt.stop()
		}
	}()

	return <-errCh
}

func (c *Controller) watch() (<-chan *Event, <-chan error) {
	eventCh := make(chan *Event)
	// On unexpected error case, controller should exit
	errCh := make(chan error, 1)

	go func() {
		defer close(eventCh)

		for _ = range time.NewTicker(5 * time.Second).C {
			newClusters := c.findClusters()
			for _, newCluster := range newClusters {
				oldCluster, ok := c.clusters[newCluster.Metadata.Name]
				// if we don't find the cluster
				if !ok {
					eventCh <- &Event{Type: kwatch.Added, Object: newCluster}
				}
				// TODO implement a compare function to do this
				if reflect.DeepEqual(oldCluster, newCluster) {
					eventCh <- &Event{Type: kwatch.Modified, Object: newCluster}
				}
			}
			// detect deleted clusters
			for _, oldCluster := range c.clusters {
				if _, ok := newClusters[oldCluster.Metadata.Name]; !ok {
					eventCh <- &Event{Type: kwatch.Deleted, Object: oldCluster}
				}
			}
		}
	}()

	return eventCh, errCh
}

func (c *Controller) handle(e *Event) error {
	c.log.Debugf("Received event: %+v", e)
	return nil
}

func (c *Controller) reconcile() {
	c.log.Debugf("begin reconciliation")
	defer c.log.Debugf("end reconciliation")

	for _, cluster := range c.findClusters() {
		c.log.Infof("  reconciling %+v", cluster)
	}
}

func (c *Controller) findClusters() map[string]*spec.Cluster {
	services := c.client.ListEtcdServices("")
	c.log.Debugf("services: %+v", services)

	clusters := make(map[string]*spec.Cluster)
	for _, s := range services {
		cluster := ranchutil.ClusterFromService(s)
		clusters[cluster.Metadata.Name] = &cluster
	}
	c.log.Debugf("clusters: %+v", clusters)
	return clusters
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
