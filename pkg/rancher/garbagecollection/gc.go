package garbagecollection

import (
	log "github.com/Sirupsen/logrus"
	rancher "github.com/rancher/go-rancher/v2"
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
		log: pkgLogger,
		client: client,
	}
}

// FullyCollect collects resources that were created before,
// but does not belong to any current running clusters.
func (gc *GC) FullyCollect() error {
	gc.log.Debug("gc.FullyCollect()")
	return nil
}
