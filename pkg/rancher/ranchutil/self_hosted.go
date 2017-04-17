package ranchutil

import (
	"fmt"
	"path"
	"strings"

	"github.com/coreos/etcd-operator/pkg/spec"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"

	rancher "github.com/rancher/go-rancher/v2"
)

const (
	shouldCheckpointAnnotation = "checkpointer.alpha.coreos.com/checkpoint" // = "true"
	varLockVolumeName          = "var-lock"
	varLockDir                 = "/var/lock"
	etcdLockPath               = "/var/lock/etcd.lock"
)

func ContainerWithSleepWaitNetwork(c *rancher.Container) {
	c.Command[len(c.Command)-1] = fmt.Sprintf("sleep 5; %s", c.Command[len(c.Command)-1])
}

func ContainerWithAddMemberCommand(c *rancher.Container, endpoints []string, name string, peerURLs []string, cs spec.ClusterSpec) {
	memberAddCommand := fmt.Sprintf("ETCDCTL_API=3 etcdctl --endpoints=%s member add %s --peer-urls=%s",
		strings.Join(endpoints, ","), name, strings.Join(peerURLs, ","))

	c.Command[len(c.Command)-1] = fmt.Sprintf("%s; %s", memberAddCommand, c.Command[len(c.Command)-1])
}

func newEtcdContainer(m *etcdutil.Member, initialCluster []string, clusterName, state, token, networkMode, theDataDir string, cs spec.ClusterSpec) *rancher.Container {
	commands := fmt.Sprintf("/usr/local/bin/etcd --data-dir=%s --name=%s --initial-advertise-peer-urls=%s "+
		"--listen-peer-urls=http://0.0.0.0:2380 --listen-client-urls=http://0.0.0.0:2379 --advertise-client-urls=%s "+
		"--initial-cluster=%s --initial-cluster-state=%s --metrics extensive",
		theDataDir, m.Name, m.PeerAddr(), m.ClientAddr(), strings.Join(initialCluster, ","), state)

	if state == "new" {
		commands = fmt.Sprintf("%s --initial-cluster-token=%s", commands, token)
	}

	c := etcdContainer(commands, cs.Version)
	c.RestartPolicy.Name = "never"
	c.Ports = nil
	//c.DataVolumes = append(c.DataVolumes, fmt.Sprintf("%s:%s", varLockVolumeName, varLockDir))
	c.Command = []string{"sh", "-ec", fmt.Sprintf("flock %s -c '%s'", etcdLockPath, commands)}
	// FIXME should be 'host'
	c.NetworkMode = networkMode
	c.Name = m.Name
	c.Labels["app"] = "etcd"
	c.Labels["name"] = m.Name
	c.Labels["cluster"] = clusterName

	SetEtcdVersion(&c, cs.Version)

	if cs.Pod != nil {
		if cs.Pod.AntiAffinity {
			ContainerWithAntiAffinity(&c, clusterName)
		}
		if len(cs.Pod.NodeSelector) != 0 {
			ContainerWithNodeSelector(&c, cs.Pod.NodeSelector)
		}
	}

	return &c
}

func NewEtcdContainer(m *etcdutil.Member, initialCluster []string, clusterName, state, token string, cs spec.ClusterSpec) *rancher.Container {
	return newEtcdContainer(m, initialCluster, clusterName, state, token, "ipsec", dataDir, cs)
}

func NewSelfHostedEtcdContainer(name string, initialCluster []string, clusterName, ns, state, token string, cs spec.ClusterSpec) *rancher.Container {
	m := &etcdutil.Member{
		Name:       name,
		Namespace:  ns,
		ClientURLs: []string{"http://$(hostname -i):2379"},
		PeerURLs:   []string{"http://$(hostname -i):2380"},
	}
	selfHostedDataDir := path.Join(etcdVolumeMountDir, ns+"-"+name)
	return newEtcdContainer(m, initialCluster, clusterName, state, token, "ipsec", selfHostedDataDir, cs)
}
