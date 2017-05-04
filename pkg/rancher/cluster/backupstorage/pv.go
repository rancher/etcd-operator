package backupstorage

import (
	"errors"
	"fmt"
	"strings"

	"github.com/coreos/etcd-operator/pkg/rancher/ranchutil"
	"github.com/coreos/etcd-operator/pkg/spec"

	rancher "github.com/rancher/go-rancher/v2"
)

type pv struct {
	client       *rancher.RancherClient
	volumeName   string
	backupPolicy spec.BackupPolicy
}

func NewPVStorage(client *rancher.RancherClient, volumeName string, backupPolicy spec.BackupPolicy) (Storage, error) {
	s := &pv{
		client:       client,
		volumeName:   volumeName,
		backupPolicy: backupPolicy,
	}
	return s, nil
}

func (s *pv) Create() error {
	return ranchutil.CreateVolume(s.client, s.volumeName, s.backupPolicy.PV.VolumeType)
}

func (s *pv) Clone(from string) error {
	fromVolumeName := from

	// translate service name into service id
	if !strings.HasPrefix(from, "1s") {
		coll, err := s.client.Service.List(&rancher.ListOpts{})
		if err != nil {
			return err
		}
		for _, service := range coll.Data {
			if service.Name == from {
				fromVolumeName = service.Id
				break
			}
		}
		if fromVolumeName == from {
			return errors.New(fmt.Sprintf("couldn't find service with name %s", from))
		}
	}

	return ranchutil.CopyVolume(s.client, fromVolumeName, s.volumeName)
}

func (s *pv) Delete() error {
	if s.backupPolicy.CleanupBackupsOnClusterDelete {
		coll, err := s.client.Volume.List(&rancher.ListOpts{
			Filters: map[string]interface{}{
				"name": s.volumeName,
			},
		})
		if err != nil {
			return err
		}
		if len(coll.Data) != 1 {
			return errors.New(fmt.Sprintf("found %d volumes with name %s", len(coll.Data), s.volumeName))
		}
		return s.client.Volume.Delete(&coll.Data[0])
	}
	return nil
}
