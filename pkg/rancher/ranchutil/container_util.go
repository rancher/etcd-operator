package ranchutil

import (
	"fmt"
	"strings"

	"github.com/coreos/etcd-operator/pkg/backup/backupapi"
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

func etcdContainer(commands, version string) rancher.Container {
	return rancher.Container{
		// TODO: fix "sleep 5".
		// Without waiting some time, there is highly probable flakes in network setup.
		Command:     []string{"/bin/sh", "-ec", fmt.Sprintf("sleep 5; %s", commands)},
		DataVolumes: []string{
		//fmt.Sprintf("etcd-data:%s", etcdVolumeMountDir),
		},
		ImageUuid: EtcdImageName(version),
		Labels: map[string]interface{}{
			"io.rancher.container.dns": "true",
		},
		Ports:         []string{"2379", "2380"},
		RestartPolicy: &rancher.RestartPolicy{Name: "never"},
	}
}

func ContainerWithAntiAffinity(c *rancher.Container, clusterName string) {
	c.Labels["io.rancher.scheduler.affinity:container_label_ne"] =
		fmt.Sprintf("cluster=%s", clusterName)
}

func ContainerWithNodeSelector(c *rancher.Container, nodeSelector map[string]string) {
	newAffinity := ""
	for k, v := range nodeSelector {
		if newAffinity != "" {
			newAffinity = newAffinity + ","
		}
		newAffinity = newAffinity + fmt.Sprintf("%s=%s", k, v)
	}

	hostAffinityLabel := "io.rancher.scheduler.affinity:host_label"
	existingAffinity, ok := c.Labels[hostAffinityLabel]
	if ok {
		c.Labels[hostAffinityLabel] = fmt.Sprintf("%s,%s", existingAffinity, newAffinity)
	} else {
		c.Labels[hostAffinityLabel] = newAffinity
	}
}

func ContainerWithSleepWaitNetwork(c *rancher.Container) {
	c.Command[len(c.Command)-1] = fmt.Sprintf("sleep 5; %s", c.Command[len(c.Command)-1])
}

func ContainerWithAddMemberCommand(c *rancher.Container, endpoints []string, name string, peerURLs []string, cs spec.ClusterSpec) {
	memberAddCommand := fmt.Sprintf("ETCDCTL_API=3 etcdctl --endpoints=%s member add %s --peer-urls=%s",
		strings.Join(endpoints, ","), name, strings.Join(peerURLs, ","))

	c.Command[len(c.Command)-1] = fmt.Sprintf("%s; %s", memberAddCommand, c.Command[len(c.Command)-1])
}

func ContainerWithRestoreCommand(c *rancher.Container, backupAddr, token, version string, m *etcdutil.Member) {
	fetchBackup := fmt.Sprintf("wget -O %s %s", backupFile, backupapi.NewBackupURL("http", backupAddr, version))
	restoreDataDir := fmt.Sprintf("ETCDCTL_API=3 etcdctl snapshot restore %[1]s"+
		" --name %[2]s"+
		" --initial-cluster %[2]s=%[3]s"+
		" --initial-cluster-token %[4]s"+
		" --initial-advertise-peer-urls %[3]s"+
		" --data-dir %[5]s", backupFile, m.Name, m.PeerAddr(), token, dataDir)

	c.Command[len(c.Command)-1] = fmt.Sprintf("%s && %s && %s",
		fetchBackup, restoreDataDir, c.Command[len(c.Command)-1])
}

func newEtcdContainer(m *etcdutil.Member, initialCluster []string, clusterName, state, token, theDataDir string, cs spec.ClusterSpec) *rancher.Container {
	commands := fmt.Sprintf("/usr/local/bin/etcd --data-dir=%s --name=%s --initial-advertise-peer-urls=%s "+
		"--listen-peer-urls=http://0.0.0.0:2380 --listen-client-urls=http://0.0.0.0:2379 --advertise-client-urls=%s "+
		"--initial-cluster=%s --initial-cluster-state=%s --metrics extensive",
		theDataDir, m.Name, m.PeerAddr(), m.ClientAddr(), strings.Join(initialCluster, ","), state)

	if state == "new" {
		commands = fmt.Sprintf("%s --initial-cluster-token=%s", commands, token)
	}

	c := etcdContainer(commands, cs.Version)
	c.Ports = nil
	//c.DataVolumes = append(c.DataVolumes, fmt.Sprintf("%s:%s", varLockVolumeName, varLockDir))
	c.Command = []string{"sh", "-ec", fmt.Sprintf("flock %s -c '%s'", etcdLockPath, commands)}
	c.Name = m.Name
	c.NetworkMode = cs.Network
	c.Labels["app"] = "etcd"
	c.Labels["name"] = m.Name
	c.Labels["cluster"] = clusterName
	// use a named volume for upgrade support
	c.DataVolumes = []string{
		fmt.Sprintf("%s:%s", m.Name, etcdVolumeMountDir),
	}

	SetEtcdVersion(&c, cs.Version)
	return &c
}

func NewEtcdContainer(m *etcdutil.Member, initialCluster []string, clusterName, state, token string, cl *spec.Cluster) *rancher.Container {
	cs := cl.Spec
	c := newEtcdContainer(m, initialCluster, clusterName, state, token, dataDir, cs)
	// add etcd environment variables to every container
	c.Environment = make(map[string]interface{})
	for k, v := range cl.Metadata.Labels {
		if strings.HasPrefix(k, "ETCD_") {
			c.Environment[k] = v
		}
	}
	if cs.Pod != nil {
		if cs.Pod.AntiAffinity {
			ContainerWithAntiAffinity(c, clusterName)
		}
		if len(cs.Pod.NodeSelector) != 0 {
			ContainerWithNodeSelector(c, cs.Pod.NodeSelector)
		}
	}
	return c
}
