package ranchutil

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	rancher "github.com/rancher/go-rancher/v2"

	log "github.com/Sirupsen/logrus"
)

const (
	rancherTimeout = 5 * time.Second
)

// ContextAwareClient is a wrapper for the rancher client which effortlessly
// switches API contexts between global (account) context and environment
// (project) context.
//
// Different API actions are available in the different contexts; as such,
// the switching is necessary to perform certain operations.
//
// We actually construct multiple clients behind the scenes, and send the user
// the appropriate client for their intended purpose.
type ContextAwareClient struct {
	clients    map[string]*rancher.RancherClient
	initClient *rancher.RancherClient
}

func NewContextAwareClient() *ContextAwareClient {
	c, err := rancher.NewRancherClient(&rancher.ClientOpts{
		Url:       os.Getenv("CATTLE_URL"),
		AccessKey: os.Getenv("CATTLE_ACCESS_KEY"),
		SecretKey: os.Getenv("CATTLE_SECRET_KEY"),
		Timeout:   rancherTimeout,
	})
	if err != nil {
		log.Fatal(err)
	}

	cac := &ContextAwareClient{
		clients:    make(map[string]*rancher.RancherClient),
		initClient: c,
	}

	u, err2 := url.Parse(c.GetOpts().Url)
	if err2 != nil {
		log.Fatal(err)
	}

	pathVars := strings.Split(u.Path, "/")
	switch len(pathVars) {
	// we were given global context (v2-beta)
	case 2:
		cac.clients["global"] = c
	// we were given environment context (v2-beta/projects/1a103)
	case 4:
		cac.clients[pathVars[3]] = c
	default:
		log.Fatalf("Can't understand API endpoint: %s", c.GetOpts().Url)
	}

	return cac
}

func (c *ContextAwareClient) createClient(id string) *rancher.RancherClient {
	// construct a URL from the initial client
	u, err := url.Parse(c.initClient.GetOpts().Url)
	if err != nil {
		log.Fatal(err)
	}
	if id != "" {
		u.Path = strings.Join([]string{strings.Split(u.Path, "/")[1], "projects", id}, "/")
	} else {
		u.Path = strings.Split(u.Path, "/")[1]
	}

	cl, err2 := rancher.NewRancherClient(&rancher.ClientOpts{
		Url:       u.String(),
		AccessKey: os.Getenv("CATTLE_ACCESS_KEY"),
		SecretKey: os.Getenv("CATTLE_SECRET_KEY"),
		Timeout:   rancherTimeout,
	})
	if err2 != nil {
		log.Fatal(err2)
	}

	c.clients[id] = cl
	return c.clients[id]
}

func (c *ContextAwareClient) getOrCreateClient(id string) *rancher.RancherClient {
	if id == "" {
		id = "global"
	}
	client, ok := c.clients[id]
	if !ok {
		return c.createClient(id)
	}
	return client
}

func (c *ContextAwareClient) Global() *rancher.RancherClient {
	return c.getOrCreateClient("")
}

func (c *ContextAwareClient) Env(id string) *rancher.RancherClient {
	return c.getOrCreateClient(id)
}

func (c *ContextAwareClient) ListEtcdServices(envId string) []*rancher.Service {
	services := []*rancher.Service{}
	allServices, err := c.Env(envId).Service.List(&rancher.ListOpts{})
	if err == nil {
		for _, s := range allServices.Data {
			if s.LaunchConfig != nil && s.LaunchConfig.Labels != nil {
				if _, ok := s.LaunchConfig.Labels["io.rancher.operator"]; ok {
					services = append(services, &s)
				}
			}
		}
	}
	return services
}

//*** This should all be eliminated once upstream objects in go-rancher are fixed ***//
//
// We have to hack around some bugs in upstream json/yaml object serialization
// where "nil values" don't get sent and consequently assume a non-nil default.

func (c *ContextAwareClient) CreateService(envId string, service *Service) {
	c.create(envId, "service", service)
}

func (c *ContextAwareClient) UpdateService(envId string, service *Service) error {
	return c.update(envId, "service", service)
}

func (c *ContextAwareClient) create(envId string, otype string, o interface{}) error {
	return c.send(envId, otype, o, "POST")
}

func (c *ContextAwareClient) get(envId string, otype string, o interface{}) error {
	return c.send(envId, otype, o, "GET")
}

func (c *ContextAwareClient) update(envId string, otype string, o interface{}) error {
	return c.send(envId, otype, o, "PUT")
}

func (c *ContextAwareClient) send(envId string, otype string, createObj interface{}, method string) error {
	b, err := json.Marshal(createObj)
	if err != nil {
		return err
	}

	globalOpts := c.Global().GetOpts()
	url := fmt.Sprintf("%s/projects/%s/%s", globalOpts.Url, envId, otype)
	req, err2 := http.NewRequest(method, url, bytes.NewBuffer(b))
	if err2 != nil {
		return err2
	}
	req.SetBasicAuth(globalOpts.AccessKey, globalOpts.SecretKey)

	log.Debugf("req: %+v", req)
	httpClient := &http.Client{}
	resp, err3 := httpClient.Do(req)
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

	log.Debugf("send(): %+v", createObj)
	return nil
}

type Service struct {
	rancher.Resource

	AccountId string `json:"accountIdlog" yaml:"account_idlog"`

	AssignServiceIpAddress bool `json:"assignServiceIpAddresslog" yaml:"assign_service_ip_addresslog"`

	CreateIndex int64 `json:"createIndexlog" yaml:"create_indexlog"`

	Created string `json:"createdlog" yaml:"createdlog"`

	CurrentScale int64 `json:"currentScalelog" yaml:"current_scalelog"`

	Data map[string]interface{} `json:"datalog" yaml:"datalog"`

	Description string `json:"descriptionlog" yaml:"descriptionlog"`

	ExternalId string `json:"externalIdlog" yaml:"external_idlog"`

	Fqdn string `json:"fqdnlog" yaml:"fqdnlog"`

	HealthState string `json:"healthStatelog" yaml:"health_statelog"`

	InstanceIds []string `json:"instanceIdslog" yaml:"instance_idslog"`

	Kind string `json:"kindlog" yaml:"kindlog"`

	LaunchConfig *rancher.LaunchConfig `json:"launchConfiglog" yaml:"launch_configlog"`

	LbConfig *rancher.LbTargetConfig `json:"lbConfiglog" yaml:"lb_configlog"`

	LinkedServices map[string]interface{} `json:"linkedServiceslog" yaml:"linked_serviceslog"`

	Metadata map[string]interface{} `json:"metadatalog" yaml:"metadatalog"`

	Name string `json:"namelog" yaml:"namelog"`

	PublicEndpoints []rancher.PublicEndpoint `json:"publicEndpointslog" yaml:"public_endpointslog"`

	RemoveTime string `json:"removeTimelog" yaml:"remove_timelog"`

	Removed string `json:"removedlog" yaml:"removedlog"`

	RetainIp bool `json:"retainIplog" yaml:"retain_iplog"`

	Scale int64 `json:"scale" yaml:"scale"`

	ScalePolicy *rancher.ScalePolicy `json:"scalePolicylog" yaml:"scale_policylog"`

	SecondaryLaunchConfigs []rancher.SecondaryLaunchConfig `json:"secondaryLaunchConfigslog" yaml:"secondary_launch_configslog"`

	SelectorContainer string `json:"selectorContainerlog" yaml:"selector_containerlog"`

	SelectorLink string `json:"selectorLinklog" yaml:"selector_linklog"`

	StackId string `json:"stackIdlog" yaml:"stack_idlog"`

	StartOnCreate bool `json:"startOnCreate" yaml:"start_on_create"`

	State string `json:"statelog" yaml:"statelog"`

	System bool `json:"systemlog" yaml:"systemlog"`

	Transitioning string `json:"transitioninglog" yaml:"transitioninglog"`

	TransitioningMessage string `json:"transitioningMessagelog" yaml:"transitioning_messagelog"`

	TransitioningProgress int64 `json:"transitioningProgresslog" yaml:"transitioning_progresslog"`

	Upgrade *rancher.ServiceUpgrade `json:"upgradelog" yaml:"upgradelog"`

	Uuid string `json:"uuidlog" yaml:"uuidlog"`

	Vip string `json:"viplog" yaml:"viplog"`
}
