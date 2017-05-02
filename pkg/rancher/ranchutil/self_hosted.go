package ranchutil

import (
	"path"

	"github.com/coreos/etcd-operator/pkg/spec"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"

	rancher "github.com/rancher/go-rancher/v2"
)

// TODO figure out more robust way to detect public IP
func NewSelfHostedEtcdContainer(name string, initialCluster []string, clusterName, ns, state, token string, cs spec.ClusterSpec) *rancher.Container {
	m := &etcdutil.Member{
		Name:       name,
		Namespace:  ns,
		ClientURLs: []string{"http://$(wget -q -O - icanhazip.com):2379"},
		PeerURLs:   []string{"http://$(wget -q -O - icanhazip.com):2380"},
	}
	selfHostedDataDir := path.Join(etcdVolumeMountDir, ns+"-"+name)
	c := newEtcdContainer(m, initialCluster, clusterName, state, token, selfHostedDataDir, cs)

	ContainerWithAntiAffinity(c, clusterName)
	if cs.Pod != nil && len(cs.Pod.NodeSelector) != 0 {
		ContainerWithNodeSelector(c, cs.Pod.NodeSelector)
	}
	return c
}
