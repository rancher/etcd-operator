package garbagecollection

import (
	"github.com/coreos/etcd-operator/pkg/rancher/ranchutil"

	log "github.com/Sirupsen/logrus"
)

const (
	NullUID = ""
)

var pkgLogger = log.WithField("pkg", "gc")

type GC struct {
	log *log.Entry

	client *ranchutil.ContextAwareClient
}

func New(client *ranchutil.ContextAwareClient) *GC {
	return &GC{
		log:    pkgLogger,
		client: client,
	}
}

// FullyCollect collects resources that were created before,
// but does not belong to any current running clusters.
func (gc *GC) FullyCollect() error {
	gc.log.Debug("gc.FullyCollect()")
	return nil
}
