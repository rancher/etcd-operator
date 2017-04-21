package rancher

import (
	"os"

	"github.com/coreos/etcd-operator/pkg/backup"
	"github.com/coreos/etcd-operator/pkg/rancher/ranchutil"
	"github.com/coreos/etcd-operator/pkg/util/constants"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"

	"github.com/Sirupsen/logrus"
	"github.com/coreos/etcd/clientv3"
	rancher "github.com/rancher/go-rancher/v2"
	"github.com/urfave/cli"
	"golang.org/x/net/context"
)

type Snapshot struct {
	client      *rancher.RancherClient
	clusterName string
}

type BackupManager struct {
	client *rancher.RancherClient
	backup *backup.Backup
}

func NewBackupManager(c *cli.Context) *backup.Backup {
	if len(c.String("cluster-name")) == 0 {
		panic("cluster-name not set")
	}
	client, err := rancher.NewRancherClient(&rancher.ClientOpts{
		Url:       os.Getenv("CATTLE_URL"),
		AccessKey: os.Getenv("CATTLE_ACCESS_KEY"),
		SecretKey: os.Getenv("CATTLE_SECRET_KEY"),
		Timeout:   ranchutil.ClientTimeout,
	})
	if err != nil {
		logrus.Fatal(err)
	}
	snapshot := Snapshot{
		client:      client,
		clusterName: c.String("cluster-name"),
	}
	return backup.New(
		snapshot,
		c.String("cluster-name"),
		*backup.ParsePolicy(c.String("backup-policy")),
		c.String("listen"))
}

func (s Snapshot) Save(lastSnapRev int64) (*etcdutil.Member, int64) {
	logrus.Info("Save()")
	coll, err := s.client.Container.List(&rancher.ListOpts{
		Filters: map[string]interface{}{
			"limit": "10000",
		},
	})
	if err != nil {
		logrus.Warningf("error listing pods: %v", err)
		return nil, lastSnapRev
	}

	var containers []rancher.Container
	for _, container := range coll.Data {
		if container.State != "running" {
			continue
		}
		if val, ok := container.Labels["cluster"]; !ok || val.(string) != s.clusterName {
			continue
		}
		containers = append(containers, container)
	}

	if len(containers) == 0 {
		logrus.Warning("no running etcd containers found")
		return nil, lastSnapRev
	}
	member, rev := getMemberWithMaxRev(containers)
	if member == nil {
		logrus.Warning("no reachable member")
		return nil, lastSnapRev
	}
	return member, rev
}

func getMemberWithMaxRev(containers []rancher.Container) (*etcdutil.Member, int64) {
	logrus.Info("getMemberWithMaxRev(%d)", len(containers))
	var member *etcdutil.Member
	maxRev := int64(0)
	for _, container := range containers {
		m := &etcdutil.Member{
			Name:     container.Name,
			Provider: "rancher",
		}
		cfg := clientv3.Config{
			Endpoints:   []string{m.ClientAddr()},
			DialTimeout: constants.DefaultDialTimeout,
		}
		etcdcli, err := clientv3.New(cfg)
		if err != nil {
			logrus.Warningf("config: %+v", cfg)
			logrus.Warningf("failed to create etcd client for pod (%v): %v", container.Name, err)
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
