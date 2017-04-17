package cluster

import (
	"fmt"

	"github.com/coreos/etcd-operator/pkg/spec"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"

	rancher "github.com/rancher/go-rancher/v2"
)

func (c *Cluster) updateMembers(known etcdutil.MemberSet) error {
	resp, err := etcdutil.ListMembers(known.ClientURLs())
	if err != nil {
		return err
	}
	members := etcdutil.MemberSet{}
	for _, m := range resp.Members {
		var name string
		if c.cluster.Spec.SelfHosted != nil {
			name = m.Name
			if len(name) == 0 || len(m.ClientURLs) == 0 {
				c.logger.Errorf("member peerURL (%s): %v", m.PeerURLs[0], errUnexpectedUnreadyMember)
				return errUnexpectedUnreadyMember
			}

			curl := m.ClientURLs[0]
			bcurl := c.cluster.Spec.SelfHosted.BootMemberClientEndpoint
			if curl == bcurl {
				return fmt.Errorf("skipping update members for self hosted cluster: waiting for the boot member (%s) to be removed...", m.Name)
			}
		} else {
			name, err = etcdutil.MemberNameFromPeerURL(m.PeerURLs[0])
			if err != nil {
				c.logger.Errorf("invalid member peerURL (%s): %v", m.PeerURLs[0], err)
				return errInvalidMemberName
			}
		}
		ct, err := etcdutil.GetCounterFromMemberName(name)
		if err != nil {
			c.logger.Errorf("invalid member name (%s): %v", name, err)
			return errInvalidMemberName
		}
		if ct+1 > c.memberCounter {
			c.memberCounter = ct + 1
		}

		members[name] = &etcdutil.Member{
			Name:       name,
			Namespace:  c.cluster.Metadata.Namespace,
			ID:         m.ID,
			ClientURLs: m.ClientURLs,
			PeerURLs:   m.PeerURLs,
			Provider:   "rancher",
		}
	}
	c.members = members
	return nil
}

func containersToMemberSet(containers []rancher.Container, selfHosted *spec.SelfHostedPolicy) etcdutil.MemberSet {
	members := etcdutil.MemberSet{}
	for _, container := range containers {
		m := &etcdutil.Member{Name: container.Name, Namespace: container.AccountId, Provider: "rancher"}
		if selfHosted != nil {
			m.ClientURLs = []string{"http://" + container.PrimaryIpAddress + ":2379"}
			m.PeerURLs = []string{"http://" + container.PrimaryIpAddress + ":2380"}
		}
		members.Add(m)
	}
	return members
}
