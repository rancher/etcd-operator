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
	log *log.Entry

	client *rancher.RancherClient
}

func New(client *rancher.RancherClient) *GC {
	return &GC{
		log:    pkgLogger,
		client: client,
	}
}

// CollectCluster collects resources that matches cluster lable, but
// does not belong to the cluster with given clusterUID
func (gc *GC) CollectCluster(cluster string, clusterUID types.UID) {
	//gc.collectResources(k8sutil.ClusterListOpt(cluster), map[types.UID]bool{clusterUID: true})
}

// FullyCollect collects resources that were created before,
// but does not belong to any current running clusters.
func (gc *GC) FullyCollect() error {
	gc.log.Debug("gc.FullyCollect()")
	return nil
}
