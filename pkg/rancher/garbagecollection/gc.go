package garbagecollection

import (
	"github.com/coreos/etcd-operator/pkg/rancher/ranchutil"

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

func newListOpts() *rancher.ListOpts {
	return &rancher.ListOpts{
		Filters: map[string]interface{}{
			"limit": "10000",
		},
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

	for _, c := range coll.Data {
		if val, ok := c.Labels["app"]; !ok || val != "etcd" {
			continue
		}
		if val, ok := c.Labels["cluster"]; !ok || val != cluster {
			continue
		}
		if err := gc.client.Env(envId).Container.Delete(&c); err != nil {
			gc.logger.Warnf("couldn't delete container: %v", err)
			continue
		}
		gc.logger.Infof("deleted container (%s)", c.Name)
	}
	//gc.collectResources(&rancher.ListOpts{}, map[types.UID]bool{clusterUID: true})
	//gc.collectResources(k8sutil.ClusterListOpt(cluster), map[types.UID]bool{clusterUID: true})
}

// FullyCollect collects resources that were created before,
// but does not belong to any current running clusters.
func (gc *GC) FullyCollect() error {
	gc.logger.Debug("gc.FullyCollect()")
	clusters, err := gc.getClusters()
	if err != nil {
		return err
	}

	coll, err := gc.client.Global().Container.List(newListOpts())
	if err != nil {
		return err
	}
	for _, c := range coll.Data {
		if val, ok := c.Labels["app"]; !ok {
			continue
		} else if val.(string) != "etcd" {
			continue
		}
		if val, ok := c.Labels["cluster"]; ok && clusters[val.(string)] {
			continue
		}
		ranchutil.SetResourceContext(&c.Resource, c.AccountId)
		if err := gc.client.Env(c.AccountId).Container.Delete(&c); err != nil {
			gc.logger.Warnf("couldn't delete container: %v", err)
			continue
		}
		gc.logger.Infof("deleted container (%s)", c.Name)
	}
	return nil
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

// func (gc *GC) collectResources(option *rancher.ListOpts, runningSet map[string]bool) {
// 	if err := gc.collectContainers(option, runningSet); err != nil {
// 		gc.logger.Errorf("gc containers failed: %v", err)
// 	}
// }

// func (gc *GC) collectContainers(option *rancher.ListOpts, runningSet map[string]bool) error {
// 	coll, err := gc.client.Container.List(option)
// 	if err != nil {
// 		return err
// 	}
// 	for _, c := range coll.Data {
// 		if val, ok := c.Labels["app"]; !ok || val != "etcd" {
// 			continue
// 		}
// 		if val, ok := c.Labels["cluster"]; ok && !runningSet[val] {
// 			if err := gc.client.Container.Delete(c); err != nil {
// 				return err
// 			}
// 			gc.logger.Infof("deleted container (%s)", c.Name)
// 		}
// 	}
// 	return nil
// }
