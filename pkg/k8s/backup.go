package k8s

import (
	"github.com/coreos/etcd-operator/pkg/backup"
	"github.com/coreos/etcd-operator/pkg/k8s/k8sutil"
	"github.com/coreos/etcd-operator/pkg/util/constants"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/pkg/api/v1"
)

type Snapshot struct {
	client      kubernetes.Interface
	clusterName string
	namespace   string
}

func NewBackupManager(c *cli.Context) *backup.Backup {
	if len(c.String("cluster-name")) == 0 {
		panic("cluster-name not set")
	}
	snapshot := Snapshot{
		client:      k8sutil.MustNewKubeClient(),
		clusterName: c.String("cluster-name"),
		namespace:   c.String("namespace"),
	}
	return backup.New(
		snapshot,
		c.String("cluster-name"),
		*backup.ParsePolicy(c.String("backup-policy")),
		c.String("listen"))
}

func (s Snapshot) Save(lastSnapRev int64) (*etcdutil.Member, int64) {
	podList, err := s.client.Core().Pods(s.namespace).List(k8sutil.ClusterListOpt(s.clusterName))
	if err != nil {
		logrus.Warningf("error listing pods: %v", err)
		return nil, lastSnapRev
	}

	var pods []*v1.Pod
	for i := range podList.Items {
		pod := &podList.Items[i]
		if pod.Status.Phase == v1.PodRunning {
			pods = append(pods, pod)
		}
	}

	if len(pods) == 0 {
		logrus.Warning("no running etcd pods found")
		return nil, lastSnapRev
	}
	member, rev := getMemberWithMaxRev(pods)
	if member == nil {
		logrus.Warning("no reachable member")
		return nil, lastSnapRev
	}
	return member, rev
}

func getMemberWithMaxRev(pods []*v1.Pod) (*etcdutil.Member, int64) {
	var member *etcdutil.Member
	maxRev := int64(0)
	for _, pod := range pods {
		m := &etcdutil.Member{
			Name:      pod.Name,
			Namespace: pod.Namespace,
		}
		cfg := clientv3.Config{
			Endpoints:   []string{m.ClientAddr()},
			DialTimeout: constants.DefaultDialTimeout,
		}
		etcdcli, err := clientv3.New(cfg)
		if err != nil {
			logrus.Warningf("config: %+v", cfg)
			logrus.Warningf("failed to create etcd client for pod (%v): %v", pod.Name, err)
			continue
		}
		defer etcdcli.Close()

		ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
		resp, err := etcdcli.Get(ctx, "/", clientv3.WithSerializable())
		cancel()
		if err != nil {
			logrus.Warningf("getMaxRev: failed to get revision from member %s (%s)", m.Name, m.ClientAddr())
			continue
		}

		logrus.Infof("getMaxRev: member %s revision (%d)", m.Name, resp.Header.Revision)
		if resp.Header.Revision > maxRev {
			maxRev = resp.Header.Revision
			member = m
		}
	}
	return member, maxRev
}
