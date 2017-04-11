package ranchutil

import (
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd-operator/pkg/spec"

	"k8s.io/client-go/pkg/api/v1"
	log "github.com/Sirupsen/logrus"
	rancher "github.com/rancher/go-rancher/v2"
)

const (
	rancherTimeout = 5 * time.Second
)

func MustNewRancherClient() *rancher.RancherClient {
	c, err := rancher.NewRancherClient(&rancher.ClientOpts{
		Url:       os.Getenv("CATTLE_URL"),
		AccessKey: os.Getenv("CATTLE_ACCESS_KEY"),
		SecretKey: os.Getenv("CATTLE_SECRET_KEY"),
		Timeout:   rancherTimeout,
	})
	if err != nil {
		log.Fatal(err)
	}
	return c
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

func opLabel(key string) string {
	return "io.rancher.operator.etcd." + key
}

func (s *Service) label(label string) string {
	if val, ok := s.LaunchConfig.Labels[label]; ok {
		return val.(string)
	}
	return ""
}

func (s *Service) labelBool(label string, def bool) bool {
	switch s.label(label) {
	case "true":
		return true
	case "false":
		return false
	default:
		return def
	}
}

func (s *Service) labelString(label string, def string) string {
	l := s.label(label)
	if l == "" {
		return def
	}
	return l
}

func (s *Service) labelInt(label string, def int) int {
	l := s.label(label)
	if val, err := strconv.Atoi(l); err == nil {
		return val
	}
	return def
}

func ClusterFromService(s *Service) *spec.Cluster {
	nodeSelector := map[string]string{}
	hostAffinities := s.label("io.rancher.scheduler.affinity:host_label")
	if hostAffinities != "" {
		for _, label := range strings.Split(hostAffinities, ",") {
			kv := strings.Split(label, "=")
			if len(kv) == 2 {
				nodeSelector[kv[0]] = kv[1]
			}
		}
	}

	return &spec.Cluster{
		Metadata: v1.ObjectMeta{
			Name: s.Name,
		},
		Spec: spec.ClusterSpec{
			Size: s.labelInt(opLabel("size"), 1),
			Version: s.labelString(opLabel("version"), "3.1.4"),
			Paused: s.labelBool(opLabel("paused"), false),
			Pod: &spec.PodPolicy{
				NodeSelector: nodeSelector,
				AntiAffinity: s.labelBool(opLabel("antiaffinity"), false),
				// TODO resource constraints?
			},
			Backup: &spec.BackupPolicy{},
			Restore: &spec.RestorePolicy{},
			SelfHosted: &spec.SelfHostedPolicy{},
			TLS: &spec.TLSPolicy{},
		},
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
				opLabel("name"):                                name,
				opLabel("scale"):                               "3",
				opLabel("version"):                             "v3.1.5",
			},
			NetworkMode: "bridge",
		},
		Name:          name,
		Scale:         0,
		StackId:       stackID,
		StartOnCreate: true,
	}
}


// TODO: replace with rancher.Service, once fixed
type Service struct {
	rancher.Resource

	AccountId string `json:"accountId,omitempty" yaml:"account_id,omitempty"`

	AssignServiceIpAddress bool `json:"assignServiceIpAddress,omitempty" yaml:"assign_service_ip_address,omitempty"`

	CreateIndex int64 `json:"createIndex,omitempty" yaml:"create_index,omitempty"`

	Created string `json:"created,omitempty" yaml:"created,omitempty"`

	CurrentScale int64 `json:"currentScale,omitempty" yaml:"current_scale,omitempty"`

	Data map[string]interface{} `json:"data,omitempty" yaml:"data,omitempty"`

	Description string `json:"description,omitempty" yaml:"description,omitempty"`

	ExternalId string `json:"externalId,omitempty" yaml:"external_id,omitempty"`

	Fqdn string `json:"fqdn,omitempty" yaml:"fqdn,omitempty"`

	HealthState string `json:"healthState,omitempty" yaml:"health_state,omitempty"`

	InstanceIds []string `json:"instanceIds,omitempty" yaml:"instance_ids,omitempty"`

	Kind string `json:"kind,omitempty" yaml:"kind,omitempty"`

	LaunchConfig *rancher.LaunchConfig `json:"launchConfig,omitempty" yaml:"launch_config,omitempty"`

	LbConfig *rancher.LbTargetConfig `json:"lbConfig,omitempty" yaml:"lb_config,omitempty"`

	LinkedServices map[string]interface{} `json:"linkedServices,omitempty" yaml:"linked_services,omitempty"`

	Metadata map[string]interface{} `json:"metadata,omitempty" yaml:"metadata,omitempty"`

	Name string `json:"name,omitempty" yaml:"name,omitempty"`

	PublicEndpoints []rancher.PublicEndpoint `json:"publicEndpoints,omitempty" yaml:"public_endpoints,omitempty"`

	RemoveTime string `json:"removeTime,omitempty" yaml:"remove_time,omitempty"`

	Removed string `json:"removed,omitempty" yaml:"removed,omitempty"`

	RetainIp bool `json:"retainIp,omitempty" yaml:"retain_ip,omitempty"`

	Scale int64 `json:"scale" yaml:"scale"`

	ScalePolicy *rancher.ScalePolicy `json:"scalePolicy,omitempty" yaml:"scale_policy,omitempty"`

	SecondaryLaunchConfigs []rancher.SecondaryLaunchConfig `json:"secondaryLaunchConfigs,omitempty" yaml:"secondary_launch_configs,omitempty"`

	SelectorContainer string `json:"selectorContainer,omitempty" yaml:"selector_container,omitempty"`

	SelectorLink string `json:"selectorLink,omitempty" yaml:"selector_link,omitempty"`

	StackId string `json:"stackId,omitempty" yaml:"stack_id,omitempty"`

	StartOnCreate bool `json:"startOnCreate,omitempty" yaml:"start_on_create,omitempty"`

	State string `json:"state,omitempty" yaml:"state,omitempty"`

	System bool `json:"system,omitempty" yaml:"system,omitempty"`

	Transitioning string `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`

	TransitioningMessage string `json:"transitioningMessage,omitempty" yaml:"transitioning_message,omitempty"`

	TransitioningProgress int64 `json:"transitioningProgress,omitempty" yaml:"transitioning_progress,omitempty"`

	Upgrade *rancher.ServiceUpgrade `json:"upgrade,omitempty" yaml:"upgrade,omitempty"`

	Uuid string `json:"uuid,omitempty" yaml:"uuid,omitempty"`

	Vip string `json:"vip,omitempty" yaml:"vip,omitempty"`
}