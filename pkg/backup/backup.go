// Copyright 2016 The etcd-operator Authors
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package backup

import (
	"encoding/json"
	"fmt"
	"log"
	"os"
	"path"
	"time"

	"github.com/coreos/etcd-operator/pkg/backup/backupapi"
	"github.com/coreos/etcd-operator/pkg/backup/env"
	"github.com/coreos/etcd-operator/pkg/backup/s3"
	"github.com/coreos/etcd-operator/pkg/spec"
	"github.com/coreos/etcd-operator/pkg/util/constants"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"

	"github.com/Sirupsen/logrus"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/coreos/etcd/clientv3"
	"golang.org/x/net/context"
)

const (
	PVBackupV1 = "v1"

	maxRecentBackupStatusCount = 10
)

type Snapshot interface {
	Save(lastSnapRevision int64) (*etcdutil.Member, int64)
}

type Backup struct {
	snapshot Snapshot

	clusterName string
	policy      spec.BackupPolicy
	listenAddr  string

	be backend

	backupNow chan chan error

	// recentBackupStatus keeps the statuses of 'maxRecentBackupStatusCount' recent backups.
	recentBackupsStatus []backupapi.BackupStatus
}

func New(snapshot Snapshot, clusterName string, policy spec.BackupPolicy, listenAddr string) *Backup {
	bdir := path.Join(constants.BackupMountDir, PVBackupV1, clusterName)
	// We created not only backup dir and but also tmp dir under it.
	// tmp dir is used to store intermediate snapshot files.
	// It will be no-op if target dir existed.
	tmpDir := path.Join(bdir, backupTmpDir)
	if err := os.MkdirAll(tmpDir, 0700); err != nil {
		panic(err)
	}

	var be backend
	switch policy.StorageType {
	case spec.BackupStorageTypePersistentVolume, spec.BackupStorageTypeDefault:
		be = &fileBackend{dir: bdir}
	case spec.BackupStorageTypeS3:
		prefix := clusterName + "/"
		s3cli, err := s3.New(os.Getenv(env.AWSS3Bucket), prefix, session.Options{
			SharedConfigState: session.SharedConfigEnable,
		})
		if err != nil {
			panic(err)
		}
		be = &s3Backend{
			dir: tmpDir,
			S3:  s3cli,
		}
	default:
		logrus.Fatalf("unsupported storage type: %v", policy.StorageType)
	}

	return &Backup{
		snapshot:    snapshot,
		clusterName: clusterName,
		policy:      policy,
		listenAddr:  listenAddr,
		be:          be,

		backupNow: make(chan chan error),
	}
}

func (b *Backup) Run() {
	go b.startHTTP()

	lastSnapRev := int64(0)
	interval := constants.DefaultSnapshotInterval
	if b.policy.BackupIntervalInSecond != 0 {
		interval = time.Duration(b.policy.BackupIntervalInSecond) * time.Second
	}

	go func() {
		for {
			<-time.After(10 * time.Second)
			err := b.be.purge(b.policy.MaxBackups)
			if err != nil {
				logrus.Errorf("fail to purge backups: %v", err)
			}
		}
	}()

	for {
		var ackchan chan error
		select {
		case <-time.After(interval):
		case ackchan = <-b.backupNow:
			logrus.Info("received a backup request")
		}

		member, rev := b.snapshot.Save(lastSnapRev)
		if rev <= lastSnapRev {
			logrus.Info("skipped creating new backup: no change since last time")
			continue
		}

		logrus.Infof("saving backup for cluster (%s)", b.clusterName)
		err := b.write(member, rev)
		if err != nil {
			logrus.Errorf("write snapshot failed: %v", err)
		} else {
			lastSnapRev = rev
		}

		if ackchan != nil {
			ackchan <- err
		}
	}
}

func (b *Backup) write(m *etcdutil.Member, rev int64) error {
	start := time.Now()

	cfg := clientv3.Config{
		Endpoints:   []string{m.ClientAddr()},
		DialTimeout: constants.DefaultDialTimeout,
	}
	etcdcli, err := clientv3.New(cfg)
	if err != nil {
		return fmt.Errorf("failed to create etcd client (%v)", err)
	}
	defer etcdcli.Close()

	ctx, cancel := context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	resp, err := etcdcli.Maintenance.Status(ctx, m.ClientAddr())
	cancel()
	if err != nil {
		return err
	}

	ctx, cancel = context.WithTimeout(context.Background(), constants.DefaultRequestTimeout)
	rc, err := etcdcli.Maintenance.Snapshot(ctx)
	if err != nil {
		return fmt.Errorf("failed to receive snapshot (%v)", err)
	}
	defer cancel()
	defer rc.Close()

	n, err := b.be.save(resp.Version, rev, rc)
	if err != nil {
		return err
	}

	bs := backupapi.BackupStatus{
		CreationTime:     time.Now().Format(time.RFC3339),
		Size:             toMB(n),
		Version:          resp.Version,
		TimeTookInSecond: int(time.Since(start).Seconds() + 1),
	}
	b.recentBackupsStatus = append(b.recentBackupsStatus, bs)
	if len(b.recentBackupsStatus) > maxRecentBackupStatusCount {
		b.recentBackupsStatus = b.recentBackupsStatus[1:]
	}

	return nil
}
func ParsePolicy(policy string) *spec.BackupPolicy {
	p := &spec.BackupPolicy{}
	if err := json.Unmarshal([]byte(policy), p); err != nil {
		log.Fatalf("fail to parse backup policy (%s): %v", policy, err)
	}
	return p
}
