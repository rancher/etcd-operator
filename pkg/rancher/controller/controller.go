package controller

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"net/http"
	"sync"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd-operator/pkg/k8s/cluster"
	rancher "github.com/rancher/go-rancher/v2"
)

type Controller struct {
	logger  *logrus.Entry
	hclient *http.Client
	rclient *rancher.RancherClient

	// TODO: combine the three cluster map.
	clusters map[string]*cluster.Cluster
	// Kubernetes resource version of the clusters
	clusterRVs map[string]string
	stopChMap  map[string]chan struct{}

	waitCluster sync.WaitGroup
}

func New(client *rancher.RancherClient) *Controller {
	return &Controller{
		logger: logrus.WithField("pkg", "controller"),
		hclient: &http.Client{
			Timeout: 5 * time.Second,
		},
		rclient: client,

		clusters:   make(map[string]*cluster.Cluster),
		clusterRVs: make(map[string]string),
		stopChMap:  map[string]chan struct{}{},
	}
}

func (c *Controller) Run() error {
	l, err := c.rclient.Project.List(&rancher.ListOpts{
		Filters: map[string]interface{}{
			"name": "swarm",
		},
	})

	if len(l.Data) != 1 {
		c.logger.Fatalf("Couldn't find swarm env")
	}

	env := l.Data[0]

	stack := NewStack("etcd", "managed by etcd operator")
	c.CreateStack(env.Id, stack)
	//c.logger.Infof("%+v", stack)

	service := NewEtcdService("etcd", stack.Id)
	c.CreateService(env.Id, service)
	//c.logger.Infof("%+v", service)

	c.periodicallyReconcile()

	return err
}

func (c *Controller) periodicallyReconcile() {
	c.reconcile()
	t := time.NewTicker(8 * time.Second)
	for _ = range t.C {
		c.reconcile()
	}
}

func (c *Controller) reconcile() {
	services := c.findServices()
	c.logger.Infof("begin reconciliation on %d services", len(services))
	for _, s := range services {
		c.logger.Infof("  reconciling (%s) %s", s.Id, s.Name)
		//container := NewEtcdContainer("etcd", service.Id)
		//c.CreateContainer(env.Id, container)
		//c.logger.Infof("%+v", container)
	}
}

func (c *Controller) findServices() []rancher.Service {
	services := []rancher.Service{}
	col, err := c.rclient.Service.List(&rancher.ListOpts{})
	if err != nil {
		return services
	}
	c.logger.Infof("found %d services total", len(col.Data))
	for _, s := range col.Data {
		if s.LaunchConfig != nil && s.LaunchConfig.Labels != nil {
			if _, ok := s.LaunchConfig.Labels["io.rancher.operator"]; ok {
				//c.logger.Infof("%s has launch config labels %+v", s.Name, s.LaunchConfig.Labels)
				services = append(services, s)
			}
		}
	}
	c.logger.Infof("found %d etcd services", len(services))
	return services
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

func configLabel(key string) string {
	return "io.rancher.operator.etcd." + key
}

func NewEtcdService(name string, stackID string) *Service {
	return &Service{
		Description: "managed by etcd operator",
		LaunchConfig: &rancher.LaunchConfig{
			ImageUuid: "docker:quay.io/coreos/etcd:v3.1.5",
			Labels: map[string]interface{}{
				"io.rancher.scheduler.affinity:container_label_ne": configLabel("name") + "=" + name,
				"io.rancher.operator":                              "etcd",
				configLabel("name"):                                name,
				configLabel("scale"):                               "3",
				configLabel("version"):                             "v3.1.5",
			},
			NetworkMode: "bridge",
		},
		Name:          name,
		Scale:         0,
		StackId:       stackID,
		StartOnCreate: true,
	}
}

func NewEtcdContainer(serviceName string, serviceID string) *rancher.Container {
	rand.Seed(time.Now().UnixNano())
	name := fmt.Sprintf("%s-%d", serviceName, rand.Int31())

	return &rancher.Container{
		Command:     []string{"/usr/local/bin/etcd"},
		Environment: map[string]interface{}{"ETCD_NAME": name},
		ImageUuid:   "docker:quay.io/coreos/etcd:v3.1.5",
		Labels: map[string]interface{}{
			"io.rancher.operator":                              "etcd",
			"io.rancher.operator.etcd.name":                    serviceName,
			"io.rancher.scheduler.affinity:container_label_ne": "io.rancher.operator.etcd.name=" + serviceName,
		},
		Name:          name,
		NetworkMode:   "bridge",
		ServiceIds:    []string{serviceID},
		StartOnCreate: true,
		StdinOpen:     true,
		Tty:           true,
	}
}

func (c *Controller) CreateStack(envId string, stack *rancher.Stack) {
	c.create(envId, "stack", stack)
}
func (c *Controller) CreateService(envId string, service *Service) {
	c.create(envId, "service", service)
}
func (c *Controller) CreateContainer(envId string, container *rancher.Container) {
	c.create(envId, "container", container)
}

func (c *Controller) create(envId string, otype string, createObj interface{}) error {
	b, err := json.Marshal(createObj)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/projects/%s/%s", c.rclient.GetOpts().Url, envId, otype)
	req, err2 := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err2 != nil {
		return err2
	}
	req.SetBasicAuth(c.rclient.GetOpts().AccessKey, c.rclient.GetOpts().SecretKey)

	c.logger.Debugf("req: %+v", req)
	resp, err3 := c.hclient.Do(req)
	if err3 != nil {
		return err3
	}
	c.logger.Debugf("resp: %+v", resp)

	defer resp.Body.Close()
	byteContent, err4 := ioutil.ReadAll(resp.Body)
	if err4 != nil {
		return err4
	}

	if len(byteContent) > 0 {
		err5 := json.Unmarshal(byteContent, createObj)
		if err5 != nil {
			return err5
		}
	}
	return nil
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
