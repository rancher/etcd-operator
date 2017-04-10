package ranchutil

import (
	rancher "github.com/rancher/go-rancher/v2"
)

func NewContainer() *rancher.Container {
	envVars := make(map[string]interface{})
	envVars["FOO"] = "BAR"

	labels := make(map[string]interface{})
	labels["foo"] = "bar"

	return &rancher.Container{
		Command: []string{
			"/usr/sbin/httpd",
			"-asdf",
		},
		DataVolumes:           []string{"etcd:/data"},
		Description:           "My Application",
		EntryPoint:            []string{"/usr/local/bin/etcd-rancher-operator"},
		Environment:           envVars,
		ImageUuid:             "docker:llparse/etcd-operator:dev",
		InstanceTriggeredStop: "stop",
		Labels:                labels,
		Name:                  "app",
		NetworkMode:           "bridge",
		StartOnCreate:         true,
		StdinOpen:             true,
		Tty:                   true,
	}
}
