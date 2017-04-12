package ranchutil

// import (
//   rancher "github.com/rancher/go-rancher/v2"
// )

// func GetClusterTPRObject(c *rancher.RancherClient, envId, name string) (*spec.Cluster, error) {
//   New(c)
//   uri := fmt.Sprintf("/apis/%s/%s/namespaces/%s/clusters/%s", spec.TPRGroup, spec.TPRVersion, ns, name)
//   b, err := restcli.Get().RequestURI(uri).DoRaw()
//   if err != nil {
//     return nil, err
//   }
//   return readOutCluster(b)
// }

// // UpdateClusterTPRObject updates the given TPR object.
// // ResourceVersion of the object MUST be set or update will fail.
// func UpdateClusterTPRObject(restcli rest.Interface, ns string, c *spec.Cluster) (*spec.Cluster, error) {
//   if len(c.Metadata.ResourceVersion) == 0 {
//     return nil, errors.New("k8sutil: resource version is not provided")
//   }
//   return updateClusterTPRObject(restcli, ns, c)
// }

// // UpdateClusterTPRObjectUnconditionally updates the given TPR object.
// // This should only be used in tests.
// func UpdateClusterTPRObjectUnconditionally(restcli rest.Interface, ns string, c *spec.Cluster) (*spec.Cluster, error) {
//   c.Metadata.ResourceVersion = ""
//   return updateClusterTPRObject(restcli, ns, c)
// }

// func updateClusterTPRObject(restcli rest.Interface, ns string, c *spec.Cluster) (*spec.Cluster, error) {
//   uri := fmt.Sprintf("/apis/%s/%s/namespaces/%s/clusters/%s", spec.TPRGroup, spec.TPRVersion, ns, c.Metadata.Name)
//   b, err := restcli.Put().RequestURI(uri).Body(c).DoRaw()
//   if err != nil {
//     return nil, err
//   }
//   return readOutCluster(b)
// }

// func readOutCluster(b []byte) (*spec.Cluster, error) {
//   cluster := &spec.Cluster{}
//   if err := json.Unmarshal(b, cluster); err != nil {
//     return nil, err
//   }
//   return cluster, nil
// }
