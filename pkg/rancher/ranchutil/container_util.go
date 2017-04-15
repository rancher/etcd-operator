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
		Command:     []string{"/bin/sh", "-ec", fmt.Sprintf("sleep 5; %s", commands)},
		DataVolumes: etcdVolumeMounts(),
		ImageUuid:   EtcdImageName(version),
		Labels:      make(map[string]interface{}),
		NetworkMode: "ipsec",
		Ports:       []string{"2379", "2380"},
	}
}

func addContainerAntiAffinity(c *rancher.Container, clusterName string) {
	c.Labels["io.rancher.scheduler.affinity:container_label_ne"] =
		fmt.Sprintf("cluster=%s", clusterName)
}

// func PodWithAntiAffinity(pod *v1.Pod, clusterName string) *v1.Pod {
//   // set pod anti-affinity with the pods that belongs to the same etcd cluster
//   affinity := v1.Affinity{
//     PodAntiAffinity: &v1.PodAntiAffinity{
//       RequiredDuringSchedulingIgnoredDuringExecution: []v1.PodAffinityTerm{
//         {
//           LabelSelector: &unversioned.LabelSelector{
//             MatchLabels: map[string]string{
//               "etcd_cluster": clusterName,
//             },
//           },
//           TopologyKey: "kubernetes.io/hostname",
//         },
//       },
//     },
//   }

//   affinityb, err := json.Marshal(affinity)
//   if err != nil {
//     panic("failed to marshal affinty struct")
//   }

//   pod.Annotations[api.AffinityAnnotationKey] = string(affinityb)
//   return pod
// }
