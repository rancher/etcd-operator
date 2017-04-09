package garbagecollection

import (
  rancher "github.com/rancher/go-rancher/v2"
  "github.com/Sirupsen/logrus"
)

const (
  NullUID = ""
)

var pkgLogger = logrus.WithField("pkg", "gc")

type GC struct {
  logger *logrus.Entry

  client *rancher.RancherClient
  env    string
}

func New(client *rancher.RancherClient, env string) *GC {
  return &GC{
    logger: pkgLogger,
    client: client,
    env:    env,
  }
}

// FullyCollect collects resources that were created before,
// but does not belong to any current running clusters.
func (gc *GC) FullyCollect() error {
  return nil
}
