package cluster

import (
	"fmt"
	"sync"

	"github.com/coreos/etcd-operator/spec"

	"github.com/Sirupsen/logrus"
	rancher "github.com/rancher/go-rancher/v2"
)

type Cluster struct {
	log     *logrus.Entry
	client  *rancher.RancherClient
	cluster *spec.Cluster

	// in memory state of the cluster
	// status is the source of truth after Cluster struct is materialized.
	status        spec.ClusterStatus
	memberCounter int

	eventCh chan *clusterEvent
	stopCh  chan struct{}
}

func New(client *rancher.RancherClient, cl *spec.Cluster, stopC <-chan struct{}, wg *sync.WaitGroup) *Cluster {
	c := &Cluster{
		log:     logrus.WithField("pkg", "cluster").WithField("cluster-name", cl.Metadata.Name),
		client:  client,
		cluster: cl,
		status:  cl.Status.Copy(),
		eventCh: make(chan *clusterEvent, 100),
		stopCh:  make(chan struct{}),
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := c.setup(); err != nil {
			c.log.Errorf("cluster failed to setup: %v", err)
			if c.status.Phase != spec.ClusterPhaseFailed {
				c.status.SetReason(err.Error())
				c.status.SetPhase(spec.ClusterPhaseFailed)
				if err := c.updateTPRStatus(); err != nil {
					c.log.Errorf("failed to update cluster phase (%v): %v", spec.ClusterPhaseFailed, err)
				}
			}
			return
		}
		c.run(stopC)
	}()

	return c
}

func (c *Cluster) setup() error {
	err := c.cluster.Spec.Validate()
	if err != nil {
		return fmt.Errorf("invalid cluster spec: %v", err)
	}

	var shouldCreateCluster bool
	switch c.status.Phase {
	case spec.ClusterPhaseNone:
		shouldCreateCluster = true
	case spec.ClusterPhaseCreating:
		return errCreatedCluster
	case spec.ClusterPhaseRunning:
		shouldCreateCluster = false

	default:
		return fmt.Errorf("unexpected cluster phase: %s", c.status.Phase)
	}

	//if b := c.cluster.Spec.Backup; b != nil && b.MaxBackups > 0 {
	//  c.bm, err = newBackupManager(c.config, c.cluster, c.log)
	//  if err != nil {
	//    return err
	//  }
	//}

	if shouldCreateCluster {
		return c.create()
	}
	return nil
}

func (c *Cluster) create() error {
	c.status.SetPhase(spec.ClusterPhaseCreating)

	if err := c.updateTPRStatus(); err != nil {
		return fmt.Errorf("cluster create: failed to update cluster phase (%v): %v", spec.ClusterPhaseCreating, err)
	}
	c.logger.Infof("creating cluster with Spec (%#v), Status (%#v)", c.cluster.Spec, c.cluster.Status)

	//c.gc.CollectCluster(c.cluster.Metadata.Name, c.cluster.Metadata.UID)

	//if c.bm != nil {
	//  if err := c.bm.setup(); err != nil {
	//    return err
	//  }
	//}

	if c.cluster.Spec.Restore == nil {
		// Note: For restore case, we don't need to create seed member,
		// and will go through reconcile loop and disaster recovery.
		if err := c.prepareSeedMember(); err != nil {
			return err
		}
	}

	if err := c.setupServices(); err != nil {
		return fmt.Errorf("cluster create: fail to create client service LB: %v", err)
	}
	return nil
}

func (c *Cluster) updateTPRStatus() error {
	if reflect.DeepEqual(c.cluster.Status, c.status) {
		return nil
	}

	newCluster := c.cluster
	newCluster.Status = c.status
	// TODO rancherutil
	newCluster, err := k8sutil.UpdateClusterTPRObject(c.config.KubeCli.Core().RESTClient(), c.cluster.Metadata.Namespace, newCluster)
	if err != nil {
		return err
	}

	c.cluster = newCluster

	return nil
}
