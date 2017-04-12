package ranchutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"time"

	rancher "github.com/rancher/go-rancher/v2"

	log "github.com/Sirupsen/logrus"
)

type API struct {
	Client     *rancher.RancherClient
	httpClient *http.Client
}

func New(c *rancher.RancherClient) *API {
	return &API{
		Client: c,
		httpClient: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (a *API) CreateStack(envId string, stack *rancher.Stack) {
	a.create(envId, "stack", stack)
}
func (a *API) CreateService(envId string, service *Service) {
	a.create(envId, "service", service)
}
func (a *API) CreateContainer(envId string, container *rancher.Container) {
	a.create(envId, "container", container)
}

func (a *API) create(envId string, otype string, createObj interface{}) error {
	b, err := json.Marshal(createObj)
	if err != nil {
		return err
	}

	url := fmt.Sprintf("%s/projects/%s/%s", a.Client.GetOpts().Url, envId, otype)
	req, err2 := http.NewRequest("POST", url, bytes.NewBuffer(b))
	if err2 != nil {
		return err2
	}
	req.SetBasicAuth(a.Client.GetOpts().AccessKey, a.Client.GetOpts().SecretKey)

	log.Debugf("req: %+v", req)
	resp, err3 := a.httpClient.Do(req)
	if err3 != nil {
		return err3
	}
	log.Debugf("resp: %+v", resp)

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

	log.Debugf("create(): %+v", createObj)
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

	StartOnCreate bool `json:"startOnCreate" yaml:"start_on_create"`

	State string `json:"state,omitempty" yaml:"state,omitempty"`

	System bool `json:"system,omitempty" yaml:"system,omitempty"`

	Transitioning string `json:"transitioning,omitempty" yaml:"transitioning,omitempty"`

	TransitioningMessage string `json:"transitioningMessage,omitempty" yaml:"transitioning_message,omitempty"`

	TransitioningProgress int64 `json:"transitioningProgress,omitempty" yaml:"transitioning_progress,omitempty"`

	Upgrade *rancher.ServiceUpgrade `json:"upgrade,omitempty" yaml:"upgrade,omitempty"`

	Uuid string `json:"uuid,omitempty" yaml:"uuid,omitempty"`

	Vip string `json:"vip,omitempty" yaml:"vip,omitempty"`
}
