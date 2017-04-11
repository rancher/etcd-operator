package garbagecollection

import (
	"github.com/Sirupsen/logrus"
	rancher "github.com/rancher/go-rancher/v2"
)

const (
	NullUID = ""
)

var pkgLogger = logrus.WithField("pkg", "gc")

type GC struct {
	logger *logrus.Entry

	client *rancher.RancherClient
}

func New(client *rancher.RancherClient) *GC {
	return &GC{
		logger: pkgLogger,
		client: client,
	}
}

// FullyCollect collects resources that were created before,
// but does not belong to any current running clusters.
func (gc *GC) FullyCollect() error {
	return nil
}
