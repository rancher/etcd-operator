package rancherutil

import (
  "os"
  "time"

  log "github.com/Sirupsen/logrus"
  rancher "github.com/rancher/go-rancher/v2"
)

const (
  rancherTimeout = 5 * time.Second
)

func MustNewRancherClient() *rancher.RancherClient {
  c, err := rancher.NewRancherClient(&rancher.ClientOpts{
    Url:       os.Getenv("CATTLE_URL"),
    AccessKey: os.Getenv("CATTLE_ACCESS_KEY"),
    SecretKey: os.Getenv("CATTLE_SECRET_KEY"),
    Timeout:   rancherTimeout,
  })
  if err != nil {
    log.Fatal(err)
  }
  return c
}
