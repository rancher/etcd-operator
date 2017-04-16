package ranchutil

import (
	"fmt"

	rancher "github.com/rancher/go-rancher/v2"
)

func etcdVolumeMounts() []string {
	return []string{
	//fmt.Sprintf("etcd-data:%s", etcdVolumeMountDir),
	}
}

func etcdContainer(commands, version string) rancher.Container {
	return rancher.Container{
		// TODO: fix "sleep 5".
		// Without waiting some time, there is highly probable flakes in network setup.
		Command:       []string{"/bin/sh", "-ec", fmt.Sprintf("sleep 5; %s", commands)},
		DataVolumes:   etcdVolumeMounts(),
		ImageUuid:     EtcdImageName(version),
		Labels:        make(map[string]interface{}),
		NetworkMode:   "ipsec",
		Ports:         []string{"2379", "2380"},
		RestartPolicy: &rancher.RestartPolicy{Name: "always"},
	}
}

func ContainerWithAntiAffinity(c *rancher.Container, clusterName string) {
	c.Labels["io.rancher.scheduler.affinity:container_label_ne"] =
		fmt.Sprintf("cluster=%s", clusterName)
}
