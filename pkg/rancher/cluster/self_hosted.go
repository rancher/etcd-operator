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
	"fmt"
	"math"
	"time"

	"github.com/coreos/etcd-operator/pkg/k8s/k8sutil"
	"github.com/coreos/etcd-operator/pkg/rancher/ranchutil"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/coreos/etcd-operator/pkg/util/retryutil"

	"github.com/pborman/uuid"
)

func (c *Cluster) addOneSelfHostedMember() error {
	c.status.AppendScalingUpCondition(c.members.Size(), c.cluster.Spec.Size)

	newMemberName := c.CreateMemberName()
	c.memberCounter++

	peerURL := "http://$(wget -q -O - icanhazip.com):2380"
	initialCluster := append(c.members.PeerURLPairs(), newMemberName+"="+peerURL)

	container := ranchutil.NewSelfHostedEtcdContainer(newMemberName, initialCluster, c.cluster.Metadata.Name, c.cluster.Metadata.Namespace, "existing", "", c.cluster.Spec)
	ranchutil.ContainerWithAddMemberCommand(container, c.members.ClientURLs(), newMemberName, []string{peerURL}, c.cluster.Spec)
	ranchutil.ContainerWithSleepWaitNetwork(container)

	_, err := c.getClient().Container.Create(container)
	if err != nil {
		return err
	}
	// wait for the new pod to start and add itself into the etcd cluster.
	oldN := c.members.Size()
	err = retryutil.Retry(5*time.Second, math.MaxInt64, func() (bool, error) {
		err = c.updateMembers(c.members)
		if err != nil {
			c.logger.Errorf("add self hosted member: fail to update members: %v", err)
			if err == errUnexpectedUnreadyMember {
				return false, nil
			}
			return false, err
		}
		if c.members.Size() > oldN {
			return true, nil
		}
		return false, nil
	})
	if err != nil {
		return err
	}

	c.logger.Infof("added a self-hosted member (%s)", newMemberName)
	return nil
}

func (c *Cluster) newSelfHostedSeedMember() error {
	newMemberName := c.CreateMemberName()
	c.memberCounter++
	initialCluster := []string{newMemberName + "=http://$(wget -q -O - icanhazip.com):2380"}

	container := ranchutil.NewSelfHostedEtcdContainer(newMemberName, initialCluster, c.cluster.Metadata.Name, c.cluster.Metadata.Namespace, "new", uuid.New(), c.cluster.Spec)
	ranchutil.ContainerWithSleepWaitNetwork(container)

	_, err := ranchutil.CreateAndWaitContainer(c.getClient(), container, 30*time.Second)
	if err != nil {
		return err
	}

	c.logger.Infof("self-hosted cluster created with seed member (%s)", newMemberName)
	return nil
}

func (c *Cluster) migrateBootMember() error {
	c.logger.Info("migrateBootMember()")
	endpoint := c.cluster.Spec.SelfHosted.BootMemberClientEndpoint

	c.logger.Infof("migrating boot member (%s)", endpoint)

	resp, err := etcdutil.ListMembers([]string{endpoint})
	if err != nil {
		return fmt.Errorf("failed to list members from boot member (%v)", err)
	}
	if len(resp.Members) != 1 {
		return fmt.Errorf("boot cluster contains more than one member")
	}
	bootMember := resp.Members[0]

	initialCluster := make([]string, 0)
	for _, purl := range bootMember.PeerURLs {
		initialCluster = append(initialCluster, fmt.Sprintf("%s=%s", bootMember.Name, purl))
	}

	// create the  member inside Kubernetes for migration
	newMemberName := fmt.Sprintf("%s-%04d", c.cluster.Metadata.Name, c.memberCounter)
	c.memberCounter++

	peerURL := "http://$(MY_POD_IP):2380"
	initialCluster = append(initialCluster, newMemberName+"="+peerURL)

	pod := k8sutil.NewSelfHostedEtcdPod(newMemberName, initialCluster, c.cluster.Metadata.Name, c.cluster.Metadata.Namespace, "existing", "", c.cluster.Spec, c.cluster.AsOwner())
	pod = k8sutil.PodWithAddMemberInitContainer(pod, []string{endpoint}, newMemberName, []string{peerURL}, c.cluster.Spec)
	pod, err = k8sutil.CreateAndWaitPod(c.config.KubeCli, c.cluster.Metadata.Namespace, pod, 30*time.Second)
	if err != nil {
		return err
	}

	go func() {
		// TODO: a shorter timeout?
		// Waiting here for cluster to get stable:
		// - etcd data are replicated;
		// - cluster TPR state has switched to "Running"
		delay := 60 * time.Second
		c.logger.Infof("wait %v before removing the boot member", delay)
		time.Sleep(delay)

		err = etcdutil.RemoveMember([]string{"http://" + pod.Status.PodIP + ":2379"}, bootMember.ID)
		if err != nil {
			c.logger.Errorf("boot member migration: failed to remove the boot member (%v)", err)
		}
	}()

	c.logger.Infof("self-hosted cluster created with boot member (%s)", endpoint)

	return nil
}
