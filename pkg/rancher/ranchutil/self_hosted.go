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

func NewEtcdContainer(m *etcdutil.Member, initialCluster []string, clusterName, state, token string, cs spec.ClusterSpec) *rancher.Container {
	commands := fmt.Sprintf("/usr/local/bin/etcd --data-dir=%s --name=%s --initial-advertise-peer-urls=%s "+
		"--listen-peer-urls=http://0.0.0.0:2380 --listen-client-urls=http://0.0.0.0:2379 --advertise-client-urls=%s "+
		"--initial-cluster=%s --initial-cluster-state=%s",
		dataDir, m.Name, m.PeerAddr(), m.ClientAddr(), strings.Join(initialCluster, ","), state)
	if state == "new" {
		commands = fmt.Sprintf("%s --initial-cluster-token=%s", commands, token)
	}

	c := etcdContainer(commands, cs.Version)
	c.RestartPolicy.Name = "never"
	c.Ports = nil
	//c.DataVolumes = append(c.DataVolumes, fmt.Sprintf("%s:%s", varLockVolumeName, varLockDir))
	c.Command = []string{"sh", "-ec", fmt.Sprintf("flock %s -c '%s'", etcdLockPath, commands)}
	// FIXME should be 'host'
	c.NetworkMode = "ipsec"
	c.Name = m.Name
	c.Labels["app"] = "etcd"
	c.Labels["name"] = m.Name
	c.Labels["cluster"] = clusterName

	SetEtcdVersion(&c, cs.Version)

	if cs.Pod.AntiAffinity {
		ContainerWithAntiAffinity(&c, clusterName)
	}

	if cs.Pod != nil {
		if cs.Pod.AntiAffinity {
			ContainerWithAntiAffinity(&c, clusterName)
		}
		// if len(cs.Pod.NodeSelector) != 0 {
		// 	ContainerWithNodeSelector(&c, cs.Pod.NodeSelector)
		// 	//pod = PodWithNodeSelector(pod, cs.Pod.NodeSelector)
		// }
	}

	return &c
}

func NewSelfHostedEtcdContainer(name string, initialCluster []string, clusterName, ns, state, token string, cs spec.ClusterSpec) *rancher.Container {
	selfHostedDataDir := path.Join(etcdVolumeMountDir, ns+"-"+name)
	commands := fmt.Sprintf("/usr/local/bin/etcd --data-dir=%s --name=%s --initial-advertise-peer-urls=http://$(hostname -i):2380 "+
		"--listen-peer-urls=http://0.0.0.0:2380 --listen-client-urls=http://0.0.0.0:2379 --advertise-client-urls=http://$(hostname -i):2379 "+
		"--initial-cluster=%s --initial-cluster-state=%s --metrics extensive",
		selfHostedDataDir, name, strings.Join(initialCluster, ","), state)

	if state == "new" {
		commands = fmt.Sprintf("%s --initial-cluster-token=%s", commands, token)
	}

	c := etcdContainer(commands, cs.Version)
	// On node reboot, there will be two copies of etcd pod: scheduled and checkpointed one.
	// Checkpointed one will start first. But then the scheduler will detect host port conflict,
	// and set the pod (in APIServer) failed. This further affects etcd service by removing the endpoints.
	// To make scheduling phase succeed, we work around by removing ports in spec.
	// However, the scheduled pod will fail when running on node because resources (e.g. host port) are taken.
	// Thus, we make etcd pod flock first before starting etcd server.
	c.Ports = nil
	//c.DataVolumes = append(c.DataVolumes, fmt.Sprintf("%s:%s", varLockVolumeName, varLockDir))
	c.Command = []string{"sh", "-ec", fmt.Sprintf("flock %s -c '%s'", etcdLockPath, commands)}
	// FIXME should be 'host'
	c.NetworkMode = "ipsec"
	c.Name = name
	c.Labels["app"] = "etcd"
	c.Labels["name"] = name
	c.Labels["cluster"] = clusterName

	SetEtcdVersion(&c, cs.Version)

	if cs.Pod.AntiAffinity {
		ContainerWithAntiAffinity(&c, clusterName)
	}
	//if cs.Pod != nil && len(cs.Pod.NodeSelector) != 0 {
	//  pod = PodWithNodeSelector(pod, cs.Pod.NodeSelector)
	//}
	return &c
}
