// Copyright 2016 The etcd-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package cluster

import (
	"errors"
	"fmt"
	"time"

	"github.com/coreos/etcd-operator/pkg/rancher/ranchutil"
	"github.com/coreos/etcd-operator/pkg/util/retryutil"

	rancher "github.com/rancher/go-rancher/v2"
)

// FIXME
// rancher doesn't have ability to upgrade bare containers,
// so we will need to implement it.
//
// If we cannot upgrade the container and retain our network identity,
// we will need to update cluster membership as part of this process.
func (c *Cluster) upgradeOneMember(memberName string) error {
	c.status.AppendUpgradingCondition(c.cluster.Spec.Version, memberName)

	coll, err := c.getClient().Container.List(&rancher.ListOpts{
		Filters: map[string]interface{}{
			"name": memberName,
		},
	})
	if err != nil {
		return err
	}
	if len(coll.Data) != 1 {
		return errors.New(fmt.Sprintf("expected one container with name %s, found %d", memberName, len(coll.Data)))
	}
	container := &coll.Data[0]

	err = c.getClient().Container.Delete(container)
	if err != nil {
		return err
	}

	c.logger.Infof("upgrading the etcd member %s from %s to %s", memberName, ranchutil.GetEtcdVersion(container), c.cluster.Spec.Version)
	container.ImageUuid = ranchutil.EtcdImageName(c.cluster.Spec.Version)
	container.Id = ""
	container.Labels["version"] = c.cluster.Spec.Version
	// unset these so ipsec doesn't go haywire
	container.PrimaryIpAddress = ""
	delete(container.Labels, "io.rancher.cni.network")
	delete(container.Labels, "io.rancher.cni.wait")
	delete(container.Labels, "io.rancher.container.ip")
	delete(container.Labels, "io.rancher.container.mac_address")
	delete(container.Labels, "io.rancher.container.uuid")
	// schedule to the same host to pick up the old data
	container.RequestedHostId = container.HostId

	err = retryutil.Retry(10*time.Second, 6, func() (bool, error) {
		if _, err := c.getClient().Container.Create(container); err != nil {
			return false, nil
		}
		return true, nil
	})
	if err != nil {
		return fmt.Errorf("fail to update the etcd member (%s): %v", memberName, err)
	}
	c.logger.Infof("finished upgrading the etcd member %v", memberName)
	return nil
}
