package resourcelock

import (
  "encoding/json"
  "fmt"

  //"k8s.io/client-go/pkg/api/errors"
  "github.com/Sirupsen/logrus"
  rancher "github.com/rancher/go-rancher/v2"
)

type RancherSettingsLock struct {
  Name        string
  Environment string
  Client      *rancher.RancherClient
  LockConfig  ResourceLockConfig
  s           *rancher.Setting
}

// Get returns the LeaderElectionRecord
func (sl *RancherSettingsLock) Get() (*LeaderElectionRecord, error) {
  logrus.Infof("Setting.ById(): %+v", sl.Name)
  s, err := sl.Client.Setting.ById(sl.Name)
  if err != nil {
    return nil, err
  }

  logrus.Infof("settings.Get(): %+v", s)
  var record LeaderElectionRecord
  // errors.NewNotFound(qualifiedResource unversioned.GroupResource, name string) *StatusError

  if err = json.Unmarshal([]byte(s.Value), &record); err != nil {
    return nil, err
  }
  return &record, nil
}

// Create attempts to create a LeaderElectionRecord
func (sl *RancherSettingsLock) Create(ler LeaderElectionRecord) error {
  recordBytes, err := json.Marshal(ler)
  if err != nil {
    return err
  }
  sl.s, err = sl.Client.Setting.Create(&rancher.Setting{
    ActiveValue: string(recordBytes),
    InDb: true,
    Name: sl.Name,
    Source: "Etcd Operator",
    Value: string(recordBytes),
  })
  return err
}

// Update will update and existing LeaderElectionRecord
func (sl *RancherSettingsLock) Update(ler LeaderElectionRecord) error {
  return nil
}

// RecordEvent is used to record events
func (sl *RancherSettingsLock) RecordEvent(string) {
  return
}

// Identity will return the locks Identity
func (sl *RancherSettingsLock) Identity() string {
  return ""
}

// Describe is used to convert details on current resource lock
// into a string
func (sl *RancherSettingsLock) Describe() string {
  return fmt.Sprintf("%v/%v", sl.Environment, sl.Name)
}
