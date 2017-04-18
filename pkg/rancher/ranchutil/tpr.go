package ranchutil

import (
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/etcd-operator/pkg/spec"

	log "github.com/Sirupsen/logrus"
	rancher "github.com/rancher/go-rancher/v2"
)

func WatchClusters(host, ns string, httpClient *http.Client, resourceVersion string) (*http.Response, error) {
	return httpClient.Get(fmt.Sprintf("%s/apis/%s/%s/namespaces/%s/clusters?watch=true&resourceVersion=%s",
		host, spec.TPRGroup, spec.TPRVersion, ns, resourceVersion))
}

func GetClusterList(client *rancher.RancherClient, ns string) (*spec.ClusterList, error) {
	log.Debug("GetClusterList()")


	/*b, err := restcli.Get().RequestURI(listClustersURI(ns)).DoRaw()
	if err != nil {
		return nil, err
	}

	clusters := &spec.ClusterList{}
	if err := json.Unmarshal(b, clusters); err != nil {
		return nil, err
	}
	return clusters, nil*/
	return nil, nil
}

func WaitEtcdTPRReady(client *rancher.RancherClient, interval, timeout time.Duration, ns string) error {
	log.Debug("WaitEtcdTPRReady()")
	/*return retryutil.Retry(interval, int(timeout/interval), func() (bool, error) {
		_, err := restcli.Get().RequestURI(listClustersURI(ns)).DoRaw()
		if err != nil {
			if apierrors.IsNotFound(err) { // not set up yet. wait more.
				return false, nil
			}
			return false, err
		}
		return true, nil
	})*/
	return nil
}

func listClustersURI(ns string) string {
	log.Debug("listClustersURI()")
	//return fmt.Sprintf("/apis/%s/%s/namespaces/%s/clusters", spec.TPRGroup, spec.TPRVersion, ns)
	return ""
}

func GetClusterTPRObject(client *rancher.RancherClient, ns, name string) (*spec.Cluster, error) {
	log.Debug("GetClusterTPRObject()")
	s, err := client.Service.ById(name)
	if err != nil {
		return nil, err
	}
	return GetClusterTPRObjectFromService(s)
}

func GetClusterTPRObjectFromService(s *rancher.Service) (*spec.Cluster, error) {
	encodedTPR, ok := s.Metadata["io.rancher.operator.tpr"]
	if !ok {
		return nil, errors.New("TPR not found")
	}
	data, err2 := base64.StdEncoding.DecodeString(encodedTPR.(string))
	if err2 != nil {
		return nil, err2
	}
	return readOutCluster(data)
}

// UpdateClusterTPRObject updates the given TPR object.
// ResourceVersion of the object MUST be set or update will fail.
func UpdateClusterTPRObject(client *rancher.RancherClient, ns string, c *spec.Cluster) (*spec.Cluster, error) {
	log.Debug("UpdateClusterTPRObject()")
	return updateClusterTPRObject(client, ns, c)
}

// UpdateClusterTPRObjectUnconditionally updates the given TPR object.
// This should only be used in tests.
func UpdateClusterTPRObjectUnconditionally(client *rancher.RancherClient, ns string, c *spec.Cluster) (*spec.Cluster, error) {
	log.Debug("UpdateClusterTPRObjectUnconditionally()")
	return updateClusterTPRObject(client, ns, c)
}

func updateClusterTPRObject(client *rancher.RancherClient, ns string, c *spec.Cluster) (*spec.Cluster, error) {
	s, err := client.Service.ById(c.Metadata.Name)
	if err != nil {
		return nil, err
	}
	data, err2 := json.Marshal(c)
	if err2 != nil {
		return nil, err2
	}

	s.Metadata["io.rancher.operator.tpr"] = base64.StdEncoding.EncodeToString(data)
	s, err = client.Service.Update(s, *s)
	if err != nil {
		return nil, err
	}
	return c, nil
}

func readOutCluster(b []byte) (*spec.Cluster, error) {
	cluster := &spec.Cluster{}
	if err := json.Unmarshal(b, cluster); err != nil {
		return nil, err
	}
	return cluster, nil
}
