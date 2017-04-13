package ranchutil

import (
	"strconv"
	"strings"

	"github.com/coreos/etcd-operator/pkg/spec"

	rancher "github.com/rancher/go-rancher/v2"
	"k8s.io/client-go/pkg/api/v1"
)

func opLabel(key string) string {
	return "io.rancher.operator.etcd." + key
}

func getLabel(s rancher.Service, label string) string {
	if s.LaunchConfig != nil {
		if val, ok := s.LaunchConfig.Labels[label]; ok {
			return val.(string)
		}
	}
	return ""
}

func labelBool(s rancher.Service, label string, def bool) bool {
	switch getLabel(s, label) {
	case "true":
		return true
	case "false":
		return false
	default:
		return def
	}
}

func labelString(s rancher.Service, label string, def string) string {
	l := getLabel(s, label)
	if l == "" {
		return def
	}
	return l
}

func labelInt(s rancher.Service, label string, def int) int {
	l := getLabel(s, label)
	if val, err := strconv.Atoi(l); err == nil {
		return val
	}
	return def
}

func ClusterFromService(s rancher.Service) spec.Cluster {
	nodeSelector := map[string]string{}
	hostAffinities := getLabel(s, "io.rancher.scheduler.affinity:host_label")
	if hostAffinities != "" {
		for _, label := range strings.Split(hostAffinities, ",") {
			kv := strings.Split(label, "=")
			if len(kv) == 2 {
				nodeSelector[kv[0]] = kv[1]
			}
		}
	}

	return spec.Cluster{
		Metadata: v1.ObjectMeta{
			Name: s.Name,
		},
		Spec: spec.ClusterSpec{
			Size:    labelInt(s, opLabel("size"), 1),
			Version: labelString(s, opLabel("version"), "3.1.4"),
			Paused:  labelBool(s, opLabel("paused"), false),
			Pod: &spec.PodPolicy{
				NodeSelector: nodeSelector,
				AntiAffinity: labelBool(s, opLabel("antiaffinity"), false),
				// TODO resource constraints?
			},
			Backup:     &spec.BackupPolicy{},
			Restore:    &spec.RestorePolicy{},
			SelfHosted: &spec.SelfHostedPolicy{},
			TLS:        &spec.TLSPolicy{},
		},
	}
}

func NewStack(name string, desc string) *rancher.Stack {
	return &rancher.Stack{
		Name:          name,
		Description:   desc,
		Group:         desc,
		StartOnCreate: true,
		System:        false,
	}
}

// FIXME delete
func NewEtcdService(name string, stackID string) *Service {
	return &Service{
		Description: "managed by etcd operator",
		LaunchConfig: &rancher.LaunchConfig{
			ImageUuid: "docker:quay.io/coreos/etcd:v3.1.5",
			Labels: map[string]interface{}{
				"io.rancher.scheduler.affinity:container_label_ne": opLabel("name") + "=" + name,
				"io.rancher.service.selector.link":                 opLabel("name") + "=" + name,
				"io.rancher.operator":                              "etcd",
				opLabel("name"):                                    name,
				opLabel("scale"):                                   "3",
				opLabel("version"):                                 "v3.1.5",
			},
			NetworkMode: "bridge",
		},
		Name:          name,
		Scale:         0,
		StackId:       stackID,
		StartOnCreate: true,
	}
}
