package garbagecollection

import (
	"sync"
	"time"

	"github.com/coreos/etcd-operator/pkg/rancher/ranchutil"
	"github.com/coreos/etcd-operator/pkg/util/retryutil"

	log "github.com/Sirupsen/logrus"
	rancher "github.com/rancher/go-rancher/v2"
	"k8s.io/client-go/pkg/types"
)

const (
	NullUID = ""
)

var pkgLogger = log.WithField("pkg", "gc")

type GC struct {
	logger *log.Entry
	client *ranchutil.ContextAwareClient
}

func New(client *ranchutil.ContextAwareClient) *GC {
	return &GC{
		logger: pkgLogger,
		client: client,
	}
}

// CollectCluster collects resources that matches cluster lable, but
// does not belong to the cluster with given clusterUID
func (gc *GC) CollectCluster(cluster, envId string, clusterUID types.UID) {
	gc.logger.Debugf("CollectCluster(): %s, %+v", cluster, clusterUID)
	coll, err := gc.client.Env(envId).Container.List(newListOpts())
	if err != nil {
		gc.logger.Warnf("list container failed: %v", err)
		return
	}

	var wg sync.WaitGroup
	for _, c := range coll.Data {
		if val, ok := c.Labels["app"]; !ok || val != "etcd" {
			continue
		}
		if val, ok := c.Labels["cluster"]; !ok || val != cluster {
			continue
		}
		wg.Add(1)
		go func(id, name string) {
			if err := gc.collectContainerById(envId, id); err == nil {
				gc.collectVolumesByName(envId, name)
			}
			wg.Done()
		}(c.Id, c.Name)
	}
	wg.Wait()
}

// FullyCollect collects resources that were created before but no longer belong
// to any running cluster.
func (gc *GC) FullyCollect() error {
	gc.logger.Debug("FullyCollect()")
	clusters, err := gc.getClusters()
	if err != nil {
		return err
	}
	coll, err := gc.client.Global().Container.List(newListOpts())
	if err != nil {
		return err
	}
	var wg sync.WaitGroup
	for _, c := range coll.Data {
		if val, ok := c.Labels["app"]; !ok {
			continue
		} else if val.(string) != "etcd" {
			continue
		}
		if val, ok := c.Labels["cluster"]; ok && clusters[val.(string)] &&
			c.State != "error" && c.State != "stopped" {
			continue
		}
		wg.Add(1)
		go func(c rancher.Container) {
			defer wg.Done()
			envId := c.AccountId
			ranchutil.SetResourceContext(&c.Resource, envId)
			// TODO we should delete container/volume independent of eachother
			if err := gc.collectContainer(&c); err == nil {
				gc.collectVolumesByName(envId, c.Name)
			}
		}(c)
	}
	wg.Wait()
	return nil
}

func (gc *GC) collectContainerById(envId, id string) error {
	c, err := gc.client.Env(envId).Container.ById(id)
	if err != nil {
		return err
	}
	return gc.collectContainer(c)
}

func (gc *GC) collectVolumesByName(envId, name string) {
	coll, err := gc.client.Env(envId).Volume.List(&rancher.ListOpts{
		Filters: map[string]interface{}{
			"name": name,
		},
	})
	if err == nil {
		gc.collectVolumes(coll)
	}
}

func (gc *GC) collectVolumes(coll *rancher.VolumeCollection) {
	for _, v := range coll.Data {
		gc.collectVolume(&v)
	}
}

func (gc *GC) collectContainer(c *rancher.Container) error {
	err := retryutil.Retry(10*time.Second, 6, func() (bool, error) {
		if err := gc.client.Env(c.AccountId).Container.Delete(c); err != nil {
			gc.logger.Debug("error collect container: %+v", err)
			gc.logger.Debug("container: %+v", c)
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		gc.logger.WithFields(log.Fields{
			"id":    c.Id,
			"name":  c.Name,
			"error": err,
		}).Warn("delete container failed")
	} else {
		gc.logger.WithFields(log.Fields{
			"id":   c.Id,
			"name": c.Name,
		}).Info("deleted container")
	}
	return err
}

func (gc *GC) collectVolume(v *rancher.Volume) error {
	err := retryutil.Retry(10*time.Second, 12, func() (bool, error) {
		if err := gc.client.Env(v.AccountId).Volume.Delete(v); err != nil {
			return false, nil
		}
		return true, nil
	})

	if err != nil {
		gc.logger.WithFields(log.Fields{
			"id":    v.Id,
			"name":  v.Name,
			"error": err,
		}).Warn("delete volume failed")
	} else {
		gc.logger.WithFields(log.Fields{
			"id":   v.Id,
			"name": v.Name,
		}).Info("deleted volume")
	}
	return err
}

func newListOpts() *rancher.ListOpts {
	return &rancher.ListOpts{
		Filters: map[string]interface{}{
			"limit": "10000",
		},
	}
}

func (gc *GC) getClusters() (map[string]bool, error) {
	services, err := ranchutil.GetEtcdServices(gc.client.Global())
	if err != nil {
		return nil, err
	}

	clusters := make(map[string]bool)
	for _, c := range services {
		clusters[c.Id] = true
	}
	return clusters, nil
}
