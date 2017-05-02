package ranchutil

import (
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/coreos/etcd-operator/pkg/spec"
	"github.com/coreos/etcd-operator/pkg/util/constants"
	"github.com/coreos/etcd-operator/pkg/util/retryutil"

	rancher "github.com/rancher/go-rancher/v2"
	"k8s.io/client-go/pkg/api/v1"
)

const (
	etcdVolumeMountDir         = "/var/etcd"
	dataDir                    = etcdVolumeMountDir + "/data"
	backupFile                 = "/var/etcd/latest.backup"
	etcdVersionAnnotationKey   = "etcd.version"
	annotationPrometheusScrape = "prometheus.io/scrape"
	annotationPrometheusPort   = "prometheus.io/port"
)

func EtcdImageName(version string) string {
	return fmt.Sprintf("docker:quay.io/coreos/etcd:v%v", version)
}

func GetEtcdVersion(c *rancher.Container) string {
	return getLabelValue(c.Labels, "version")
}

func SetEtcdVersion(c *rancher.Container, version string) {
	c.Labels["version"] = version
}

func BackupServiceAddr(serviceName, stackName string) string {
	return fmt.Sprintf("%s:%d", BackupServiceName(serviceName, stackName), constants.DefaultBackupPodHTTPPort)
}

func BackupServiceName(serviceName, stackName string) string {
	return fmt.Sprintf("%s-backup.%s", serviceName, stackName)
}

// CreateAndWaitPod is a workaround for self hosted and util for testing.
// We should eventually get rid of this in critical code path and move it to test util.
func CreateAndWaitContainer(client *rancher.RancherClient, c *rancher.Container, timeout time.Duration) (*rancher.Container, error) {
	var err error
	c, err = client.Container.Create(c)
	if err != nil {
		return nil, err
	}

	interval := 3 * time.Second
	var retContainer *rancher.Container
	retryutil.Retry(interval, int(timeout/(interval)), func() (bool, error) {
		retContainer, err = client.Container.ById(c.Id)
		if err != nil {
			return false, err
		}
		switch retContainer.State {
		case "running":
			return true, nil
		default:
			return false, nil
		}
	})

	return retContainer, nil
}

func GetContainerNames(containers []rancher.Container) []string {
	res := []string{}
	if len(containers) == 0 {
		return nil
	}
	for _, c := range containers {
		res = append(res, c.Name)
	}
	return res
}

func opLabel(key string) string {
	return "io.rancher.operator.etcd." + key
}

func getLabelValue(labels map[string]interface{}, name string) string {
	if value, ok := labels[name]; ok {
		return value.(string)
	}
	return ""
}

func getServiceLabelValue(s rancher.Service, name string) string {
	if s.LaunchConfig != nil {
		return getLabelValue(s.LaunchConfig.Labels, name)
	}
	return ""
}

func labelBool(s rancher.Service, label string, def bool) bool {
	switch getServiceLabelValue(s, label) {
	case "true":
		return true
	case "false":
		return false
	default:
		return def
	}
}

func labelString(s rancher.Service, label string, def string) string {
	l := getServiceLabelValue(s, label)
	if l == "" {
		return def
	}
	return l
}

func labelStringMap(s rancher.Service, label string, def map[string]string) map[string]string {
	m := make(map[string]string)
	for _, entry := range strings.Split(labelString(s, label, ""), ",") {
		kv := strings.Split(entry, "=")
		if len(kv) == 2 {
			m[kv[0]] = kv[1]
		}
	}
	if len(m) == 0 {
		return def
	}
	return m
}

func labelInt(s rancher.Service, label string, def int) int {
	l := getServiceLabelValue(s, label)
	if val, err := strconv.Atoi(l); err == nil {
		return val
	}
	return def
}

func labelDuration(s rancher.Service, label string, def time.Duration) time.Duration {
	l := getServiceLabelValue(s, label)
	if val, err := time.ParseDuration(l); err == nil {
		return val
	}
	return def
}

func getPodPolicy(s rancher.Service) *spec.PodPolicy {
	return &spec.PodPolicy{
		AntiAffinity: labelBool(s, opLabel("antiaffinity"), false),
		NodeSelector: labelStringMap(s, opLabel("nodeselector"), map[string]string{}),
	}
}

func getBackupPolicy(s rancher.Service) *spec.BackupPolicy {
	if !labelBool(s, opLabel("backup"), false) {
		return nil
	}

	backupInterval := int(labelDuration(s, opLabel("backup.interval"), 1800*time.Second) / time.Second)
	bp := &spec.BackupPolicy{
		BackupIntervalInSecond:        backupInterval,
		MaxBackups:                    labelInt(s, opLabel("backup.count"), 48),
		CleanupBackupsOnClusterDelete: labelBool(s, opLabel("backup.delete"), false),
	}
	switch labelString(s, opLabel("backup.storage.type"), "") {
	case spec.BackupStorageTypePersistentVolume:
		bp.StorageType = spec.BackupStorageTypePersistentVolume
		bp.PV = &spec.PVSource{
			VolumeSizeInMB: labelInt(s, opLabel("backup.storage.size"), 1024),
			VolumeType:     labelString(s, opLabel("backup.storage.driver"), ""),
		}
	case spec.BackupStorageTypeS3:
		bp.StorageType = spec.BackupStorageTypeS3
		bp.S3 = &spec.S3Source{}
	default:
		bp.StorageType = spec.BackupStorageTypeDefault
	}
	return bp
}

func getSelfHostedPolicy(s rancher.Service) *spec.SelfHostedPolicy {
	if !labelBool(s, opLabel("selfhosted"), false) {
		return nil
	}
	return &spec.SelfHostedPolicy{}
}

func ClusterFromService(s rancher.Service, stackName string) spec.Cluster {
	cluster, err := GetClusterTPRObjectFromService(&s)
	if err != nil {
		cluster = &spec.Cluster{
			Metadata: v1.ObjectMeta{
				Labels: map[string]string{
					"serviceName": s.Name,
					"stackId":     s.StackId,
					"stackName":   stackName,
				},
				Name:      s.Id,
				Namespace: s.AccountId,
			},
		}
	}
	// overlay the spec with label values
	cluster.Spec = spec.ClusterSpec{
		Size:    labelInt(s, opLabel("size"), 1),
		Version: labelString(s, opLabel("version"), "3.1.4"),
		Paused:  labelBool(s, opLabel("paused"), false),
		Network: labelString(s, opLabel("network"), "host"),
		Pod:     getPodPolicy(s),
		Backup:  getBackupPolicy(s),
		// must be nil if not set, don't create empty object
		//Restore:    &spec.RestorePolicy{},
		SelfHosted: getSelfHostedPolicy(s),
		//TLS: &spec.TLSPolicy{},
	}
	return *cluster
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
