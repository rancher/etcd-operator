package backupstorage

import (
	"errors"
	"fmt"

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
	// TODO create volume? doesn't really matter..
	return nil
	//return k8sutil.CreateAndWaitPVC(s.kubecli, s.clusterName, s.namespace, s.pvProvisioner, s.backupPolicy.PV.VolumeSizeInMB)
}

func (s *pv) Clone(from string) error {
	// TODO copy volume
	//return k8sutil.CopyVolume(s.kubecli, from, s.clusterName, s.namespace)
	return nil
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
