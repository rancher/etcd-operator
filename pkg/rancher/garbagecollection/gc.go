package garbagecollection

import (
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
	client *rancher.RancherClient
}

func New(client *rancher.RancherClient) *GC {
	return &GC{
		logger: pkgLogger,
		client: client,
	}
}

// CollectCluster collects resources that matches cluster lable, but
// does not belong to the cluster with given clusterUID
func (gc *GC) CollectCluster(cluster string, clusterUID types.UID) {
	gc.logger.Debugf("CollectCluster(): %s, %+v", cluster, clusterUID)
	coll, err := gc.client.Container.List(&rancher.ListOpts{})
	if err != nil {
		gc.logger.Warnf("list container failed: %v", err)
		return
	}

	for _, c := range coll.Data {
		if val, ok := c.Labels["cluster"]; !ok || val != cluster {
			continue
		}
		if err := gc.client.Container.Delete(&c); err != nil {
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
	return nil
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
