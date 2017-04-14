package ranchutil

import (
	"encoding/json"
	"fmt"
	"net/http"
	"time"

	"github.com/coreos/etcd-operator/pkg/spec"

	rancher "github.com/rancher/go-rancher/v2"
)

func WatchClusters(host, ns string, httpClient *http.Client, resourceVersion string) (*http.Response, error) {
	return httpClient.Get(fmt.Sprintf("%s/apis/%s/%s/namespaces/%s/clusters?watch=true&resourceVersion=%s",
		host, spec.TPRGroup, spec.TPRVersion, ns, resourceVersion))
}

func GetClusterList(client *rancher.RancherClient, ns string) (*spec.ClusterList, error) {
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
	//return fmt.Sprintf("/apis/%s/%s/namespaces/%s/clusters", spec.TPRGroup, spec.TPRVersion, ns)
	return ""
}

func GetClusterTPRObject(client *rancher.RancherClient, ns, name string) (*spec.Cluster, error) {
	/*uri := fmt.Sprintf("/apis/%s/%s/namespaces/%s/clusters/%s", spec.TPRGroup, spec.TPRVersion, ns, name)
	b, err := restcli.Get().RequestURI(uri).DoRaw()
	if err != nil {
		return nil, err
	}
	return readOutCluster(b)*/
	return nil, nil
}

// UpdateClusterTPRObject updates the given TPR object.
// ResourceVersion of the object MUST be set or update will fail.
func UpdateClusterTPRObject(client *rancher.RancherClient, ns string, c *spec.Cluster) (*spec.Cluster, error) {
	return updateClusterTPRObject(client, ns, c)
}

// UpdateClusterTPRObjectUnconditionally updates the given TPR object.
// This should only be used in tests.
func UpdateClusterTPRObjectUnconditionally(client *rancher.RancherClient, ns string, c *spec.Cluster) (*spec.Cluster, error) {
	return updateClusterTPRObject(client, ns, c)
}

func updateClusterTPRObject(client *rancher.RancherClient, ns string, c *spec.Cluster) (*spec.Cluster, error) {
	// get the service by c.Metadata.Name (uuid)
	s, err := client.Service.ById(c.Metadata.Name)
	if err != nil {
		return nil, err
	}
	// TODO serialize cluster object nad put in service label io.rancher.operator.tpr
	s.LaunchConfig.Labels["io.rancher.operator.tpr"] = "thisisatest!!!!"
	s, err = client.Service.Update(s, nil)
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
