package ranchutil

import (
	"net/url"
	"os"
	"strings"
	"time"

	rancher "github.com/rancher/go-rancher/v2"

	log "github.com/Sirupsen/logrus"
)

const (
	ClientTimeout = 5 * time.Second
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
	u, err := url.Parse(os.Getenv("CATTLE_URL"))
	if err != nil {
		log.Fatal(err)
	}
	pathVars := strings.Split(u.Path, "/")
	// force v2-beta API
	if len(pathVars) >= 2 {
		pathVars[1] = "v2-beta"
	}
	// append /schemas to path so resp X-API-Schemas header returns correct endpoint
	if pathVars[len(pathVars)-1] != "schemas" {
		pathVars = append(pathVars, "schemas")
	}
	u.Path = strings.Join(pathVars, "/")

	c, err := rancher.NewRancherClient(&rancher.ClientOpts{
		Url:       u.String(),
		AccessKey: os.Getenv("CATTLE_ACCESS_KEY"),
		SecretKey: os.Getenv("CATTLE_SECRET_KEY"),
		Timeout:   ClientTimeout,
	})
	if err != nil {
		log.Fatal(err)
	}

	cac := &ContextAwareClient{
		clients: map[string]*rancher.RancherClient{
			getContextFromURL(c.GetOpts().Url): c,
		},
		initClient: c,
	}

	return cac
}

func (c *ContextAwareClient) createClient(id string) *rancher.RancherClient {
	// construct a URL from the initial client
	u, err := url.Parse(c.initClient.GetOpts().Url)
	if err != nil {
		log.Fatal(err)
	}
	switch id {
	case "global":
		u.Path = strings.Join([]string{strings.Split(u.Path, "/")[1], "schemas"}, "/")
	default:
		u.Path = strings.Join([]string{strings.Split(u.Path, "/")[1], "projects", id, "schemas"}, "/")
	}

	cl, err2 := rancher.NewRancherClient(&rancher.ClientOpts{
		Url:       u.String(),
		AccessKey: os.Getenv("CATTLE_ACCESS_KEY"),
		SecretKey: os.Getenv("CATTLE_SECRET_KEY"),
		Timeout:   ClientTimeout,
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
	return c.getOrCreateClient("global")
}

func (c *ContextAwareClient) Env(id string) *rancher.RancherClient {
	return c.getOrCreateClient(id)
}

func getContextFromURL(u string) string {
	context := "global"
	if v, err := url.Parse(u); err == nil {
		pathVars := strings.Split(v.Path, "/")
		if len(pathVars) >= 4 && pathVars[2] == "projects" {
			context = pathVars[3]
		}
	}
	return context
}

func updateContextURL(current, newContext string) string {
	currentContext := "unknown"
	newURL := current
	if v, err := url.Parse(current); err == nil {
		pathVars := strings.Split(v.Path, "/")
		// deduce current context and strip from path
		apiVersion := pathVars[1]
		if len(pathVars) >= 4 && pathVars[2] == "projects" {
			currentContext = pathVars[3]
			pathVars = pathVars[4:]
		} else {
			currentContext = "global"
			pathVars = pathVars[2:]
		}
		if currentContext != newContext {
			switch newContext {
			case "global":
				pathVars = append([]string{"", apiVersion}, pathVars...)
			default:
				pathVars = append([]string{"", apiVersion, "projects", newContext}, pathVars...)
			}
			v.Path = strings.Join(pathVars, "/")
			newURL = v.String()
		}
	}
	return newURL
}

// If we retrieved an object with one client, we need to adjust hyperlinks in
// order to modify/delete it with another client.
func SetResourceContext(r *rancher.Resource, envId string) {
	for name, value := range r.Links {
		r.Links[name] = updateContextURL(value, envId)
	}
	for name, value := range r.Actions {
		r.Actions[name] = updateContextURL(value, envId)
	}
}

func (c *ContextAwareClient) ListEtcdServices(envId string) ([]rancher.Service, error) {
	return GetEtcdServices(c.Env(envId))
}

func GetEtcdServices(c *rancher.RancherClient) ([]rancher.Service, error) {
	allServices, err := c.Service.List(&rancher.ListOpts{
		Filters: map[string]interface{}{
			"limit": "10000",
		},
	})
	if err != nil {
		return nil, err
	}

	services := []rancher.Service{}
	for _, s := range allServices.Data {
		if s.LaunchConfig != nil && s.LaunchConfig.Labels != nil {
			if val, ok := s.LaunchConfig.Labels["io.rancher.operator"]; ok && val == "etcd" {
				services = append(services, s)
			}
		}
	}
	return services, nil
}
