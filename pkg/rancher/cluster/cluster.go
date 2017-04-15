package cluster

import (
	"fmt"
	"math"
	"reflect"
	"sync"
	"time"

	"github.com/coreos/etcd-operator/pkg/backup/s3/s3config"
	"github.com/coreos/etcd-operator/pkg/k8s/k8sutil"
	"github.com/coreos/etcd-operator/pkg/rancher/garbagecollection"
	"github.com/coreos/etcd-operator/pkg/rancher/ranchutil"
	"github.com/coreos/etcd-operator/pkg/spec"
	"github.com/coreos/etcd-operator/pkg/util/etcdutil"
	"github.com/coreos/etcd-operator/pkg/util/retryutil"

	"github.com/Sirupsen/logrus"
	"github.com/pborman/uuid"
	rancher "github.com/rancher/go-rancher/v2"
	"k8s.io/client-go/kubernetes"
	apierrors "k8s.io/client-go/pkg/api/errors"
	"k8s.io/client-go/pkg/api/v1"
)

var (
	reconcileInterval         = 8 * time.Second
	podTerminationGracePeriod = int64(5)
)

type clusterEventType string

const (
	eventDeleteCluster clusterEventType = "Delete"
	eventModifyCluster clusterEventType = "Modify"
)

type clusterEvent struct {
	typ     clusterEventType
	cluster *spec.Cluster
}

type Config struct {
	PVProvisioner string
	s3config.S3Context

	KubeCli kubernetes.Interface
	Client  *rancher.RancherClient
}

type Cluster struct {
	logger *logrus.Entry

	config Config

	cluster *spec.Cluster

	// in memory state of the cluster
	// status is the source of truth after Cluster struct is materialized.
	status        spec.ClusterStatus
	memberCounter int

	eventCh chan *clusterEvent
	stopCh  chan struct{}

	// members repsersents the members in the etcd cluster.
	// the name of the member is the the name of the pod the member
	// process runs in.
	members etcdutil.MemberSet

	bm        *backupManager
	backupDir string

	gc *garbagecollection.GC
}

func (c Cluster) Get() *spec.Cluster {
	return c.cluster
}

func New(config Config, cl *spec.Cluster, stopC <-chan struct{}, wg *sync.WaitGroup) *Cluster {
	c := &Cluster{
		logger:  logrus.WithField("pkg", "cluster").WithField("cluster-name", cl.Metadata.Name),
		config:  config,
		cluster: cl,
		eventCh: make(chan *clusterEvent, 100),
		stopCh:  make(chan struct{}),
		status:  cl.Status.Copy(),
		gc:      garbagecollection.New(config.Client),
	}

	wg.Add(1)
	go func() {
		defer wg.Done()

		if err := c.setup(); err != nil {
			c.logger.Errorf("cluster failed to setup: %v", err)
			if c.status.Phase != spec.ClusterPhaseFailed {
				c.status.SetReason(err.Error())
				c.status.SetPhase(spec.ClusterPhaseFailed)
				if err := c.updateTPRStatus(); err != nil {
					c.logger.Errorf("failed to update cluster phase (%v): %v", spec.ClusterPhaseFailed, err)
				}
			}
			return
		}
		c.run(stopC)
	}()

	return c
}

func (c *Cluster) setup() error {
	err := c.cluster.Spec.Validate()
	if err != nil {
		return fmt.Errorf("invalid cluster spec: %v", err)
	}

	var shouldCreateCluster bool
	switch c.status.Phase {
	case spec.ClusterPhaseNone:
		shouldCreateCluster = true
	case spec.ClusterPhaseCreating:
		return errCreatedCluster
	case spec.ClusterPhaseRunning:
		shouldCreateCluster = false

	default:
		return fmt.Errorf("unexpected cluster phase: %s", c.status.Phase)
	}

	//if b := c.cluster.Spec.Backup; b != nil && b.MaxBackups > 0 {
	//  c.bm, err = newBackupManager(c.config, c.cluster, c.log)
	//  if err != nil {
	//    return err
	//  }
	//}

	if shouldCreateCluster {
		return c.create()
	}
	return nil
}

func (c *Cluster) create() error {
	c.status.SetPhase(spec.ClusterPhaseCreating)

	if err := c.updateTPRStatus(); err != nil {
		return fmt.Errorf("cluster create: failed to update cluster phase (%v): %v", spec.ClusterPhaseCreating, err)
	}
	c.logger.Infof("creating cluster with Spec (%#v), Status (%#v)", c.cluster.Spec, c.cluster.Status)

	c.gc.CollectCluster(c.cluster.Metadata.Name, c.cluster.Metadata.UID)

	if c.bm != nil {
		if err := c.bm.setup(); err != nil {
			return err
		}
	}

	if c.cluster.Spec.Restore == nil {
		// Note: For restore case, we don't need to create seed member,
		// and will go through reconcile loop and disaster recovery.
		if err := c.prepareSeedMember(); err != nil {
			return err
		}
	}

	return nil
}

func (c *Cluster) prepareSeedMember() error {
	c.logger.Infof("prepareSeedMember()")
	c.status.AppendScalingUpCondition(0, c.cluster.Spec.Size)

	var err error
	if sh := c.cluster.Spec.SelfHosted; sh != nil {
		if len(sh.BootMemberClientEndpoint) == 0 {
			err = c.newSelfHostedSeedMember()
		} else {
			err = c.migrateBootMember()
		}
	} else {
		err = c.bootstrap()
	}
	if err != nil {
		return err
	}

	c.status.Size = 1
	return nil
}

func (c *Cluster) Update(cl *spec.Cluster) {
	c.send(&clusterEvent{
		typ:     eventModifyCluster,
		cluster: cl,
	})
}

func (c *Cluster) Delete() {
	c.send(&clusterEvent{typ: eventDeleteCluster})
}

func (c *Cluster) send(ev *clusterEvent) {
	select {
	case c.eventCh <- ev:
		l, ecap := len(c.eventCh), cap(c.eventCh)
		if l > int(float64(ecap)*0.8) {
			c.logger.Warningf("eventCh buffer is almost full [%d/%d]", l, ecap)
		}
	case <-c.stopCh:
	}
}

func (c *Cluster) run(stopC <-chan struct{}) {
	clusterFailed := false

	defer func() {
		if clusterFailed {
			c.reportFailedStatus()

			c.logger.Infof("deleting the failed cluster")
			c.delete()
		}

		close(c.stopCh)
	}()

	c.status.SetPhase(spec.ClusterPhaseRunning)
	if err := c.updateTPRStatus(); err != nil {
		c.logger.Warningf("failed to update TPR status: %v", err)
	}
	c.logger.Infof("start running...")

	var rerr error
	for {
		select {
		case <-stopC:
			return
		case event := <-c.eventCh:
			switch event.typ {
			case eventModifyCluster:
				if isSpecEqual(event.cluster.Spec, c.cluster.Spec) {
					break
				}
				// TODO: we can't handle another upgrade while an upgrade is in progress
				c.logger.Infof("spec update: from: %v to: %v", c.cluster.Spec, event.cluster.Spec)
				c.cluster = event.cluster

			case eventDeleteCluster:
				c.logger.Infof("cluster is deleted by the user")
				clusterFailed = true
				return
			}

		case <-time.After(reconcileInterval):
			if c.cluster.Spec.Paused {
				c.status.PauseControl()
				c.logger.Infof("control is paused, skipping reconcilation")
				continue
			} else {
				c.status.Control()
			}

			running, pending, err := c.pollContainers()
			c.logger.Debugf("skip reconciliation: running (%v), pending (%v)", ranchutil.GetContainerNames(running), ranchutil.GetContainerNames(pending))
			if err != nil {
				c.logger.Errorf("fail to poll containers: %v", err)
			}

			if len(pending) > 0 {
				// Pod startup might take long, e.g. pulling image. It would deterministically become running or succeeded/failed later.
				c.logger.Infof("skip reconciliation: running (%v), pending (%v)", ranchutil.GetContainerNames(running), ranchutil.GetContainerNames(pending))
				continue
			}
			if len(running) == 0 {
				c.logger.Warningf("all etcd pods are dead. Trying to recover from a previous backup")
				rerr = c.disasterRecovery(nil)
				if rerr != nil {
					c.logger.Errorf("fail to do disaster recovery: %v", rerr)
				}
				// On normal recovery case, we need backoff. On error case, this could be either backoff or leading to cluster delete.
				break
			}

			// On controller restore, we could have "members == nil"
			if rerr != nil || c.members == nil {
				rerr = c.updateMembers(containersToMemberSet(running, c.cluster.Spec.SelfHosted))
				if rerr != nil {
					c.logger.Errorf("failed to update members: %v", rerr)
					break
				}
			}
			rerr = c.reconcile(running)
			if rerr != nil {
				c.logger.Errorf("failed to reconcile: %v", rerr)
				break
			}

			if err := c.updateLocalBackupStatus(); err != nil {
				c.logger.Warningf("failed to update local backup service status: %v", err)
			}
			c.updateMemberStatus(running)
			if err := c.updateTPRStatus(); err != nil {
				c.logger.Warningf("failed to update TPR status: %v", err)
			}
		}

		if isFatalError(rerr) {
			clusterFailed = true
			c.status.SetReason(rerr.Error())

			c.logger.Errorf("cluster failed: %v", rerr)
			return
		}
	}
}

func isSpecEqual(s1, s2 spec.ClusterSpec) bool {
	if s1.Size != s2.Size || s1.Paused != s2.Paused || s1.Version != s2.Version {
		return false
	}
	return true
}

func (c *Cluster) startSeedMember(recoverFromBackup bool) error {
	m := &etcdutil.Member{
		Name:      etcdutil.CreateMemberName(c.cluster.Metadata.Name, c.memberCounter),
		Namespace: c.cluster.Metadata.Namespace,
	}
	ms := etcdutil.NewMemberSet(m)
	if err := c.createPod(ms, m, "new", recoverFromBackup); err != nil {
		return fmt.Errorf("failed to create seed member (%s): %v", m.Name, err)
	}
	c.memberCounter++
	c.members = ms
	c.logger.Infof("cluster created with seed member (%s)", m.Name)
	return nil
}

// bootstrap creates the seed etcd member for a new cluster.
func (c *Cluster) bootstrap() error {
	c.logger.Infof("bootstrap()")
	return c.startSeedMember(false)
}

// recover recovers the cluster by creating a seed etcd member from a backup.
func (c *Cluster) recover() error {
	return c.startSeedMember(true)
}

func (c *Cluster) delete() {
	c.gc.CollectCluster(c.cluster.Metadata.Name, garbagecollection.NullUID)

	if c.bm == nil {
		return
	}

	if err := c.bm.cleanup(); err != nil {
		c.logger.Errorf("cluster deletion: backup manager failed to cleanup: %v", err)
	}
}

func (c *Cluster) deleteClientServiceLB() error {
	err := c.config.KubeCli.Core().Services(c.cluster.Metadata.Namespace).Delete(k8sutil.ClientServiceName(c.cluster.Metadata.Name), nil)
	if err != nil {
		if !k8sutil.IsKubernetesResourceNotFoundError(err) {
			return err
		}
	}
	err = c.config.KubeCli.Core().Services(c.cluster.Metadata.Namespace).Delete(c.cluster.Metadata.Name, nil)
	if err != nil {
		if !k8sutil.IsKubernetesResourceNotFoundError(err) {
			return err
		}
	}
	return nil
}

func (c *Cluster) createPod(members etcdutil.MemberSet, m *etcdutil.Member, state string, needRecovery bool) error {
	token := ""
	if state == "new" {
		token = uuid.New()
	}

	pod := k8sutil.NewEtcdPod(m, members.PeerURLPairs(), c.cluster.Metadata.Name, state, token, c.cluster.Spec, c.cluster.AsOwner())
	if needRecovery {
		k8sutil.AddRecoveryToPod(pod, c.cluster.Metadata.Name, token, m, c.cluster.Spec)
	}
	_, err := c.config.KubeCli.Core().Pods(c.cluster.Metadata.Namespace).Create(pod)
	if err != nil {
		return err
	}
	return nil
}

func (c *Cluster) removePodAndService(name string) error {
	ns := c.cluster.Metadata.Namespace
	err := c.config.KubeCli.Core().Services(ns).Delete(name, nil)
	if err != nil {
		if !k8sutil.IsKubernetesResourceNotFoundError(err) {
			return err
		}
	}

	opts := v1.NewDeleteOptions(podTerminationGracePeriod)
	err = c.config.KubeCli.Core().Pods(ns).Delete(name, opts)
	if err != nil {
		if !k8sutil.IsKubernetesResourceNotFoundError(err) {
			return err
		}
	}
	return nil
}

func containerHasLabel(c *rancher.Container, name, value string) bool {
	if val, ok := c.Labels[name]; ok {
		if val.(string) == value {
			return true
		}
	}
	return false
}

func (c *Cluster) pollContainers() (running, pending []*rancher.Container, err error) {
	containerColl, err := c.config.Client.Container.List(&rancher.ListOpts{})
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list running containers: %v", err)
	}

	// get the hosts into a map so we can set primaryIpAddress for networkMode=host containers
	hostColl, err2 := c.config.Client.Host.List(&rancher.ListOpts{})
	if err2 != nil {
		return nil, nil, fmt.Errorf("failed to list hosts: %v", err2)
	}
	hosts := make(map[string]rancher.Host)
	for _, host := range hostColl.Data {
		hosts[host.Id] = host
	}

	for _, n := range containerColl.Data {
		// ignore containers not belonging to this service
		// we ideally would be able to filter the List() operation
		if !containerHasLabel(&n, "cluster", c.cluster.Metadata.Name) {
			continue
		}
		if n.NetworkMode == "host" {
			if h, ok := hosts[n.HostId]; ok {
				n.PrimaryIpAddress = h.AgentIpAddress
			}
		}
		switch n.State {
		case "running":
			running = append(running, &n)
		default:
			pending = append(pending, &n)
		}

		c.logger.Debugf("container: %s", n.Name)
	}
	return running, pending, nil
}

func (c *Cluster) pollPods() (running, pending []*v1.Pod, err error) {
	if c.config.KubeCli == nil {
		return nil, nil, fmt.Errorf("no kubecli")
	}
	podList, err := c.config.KubeCli.Core().Pods(c.cluster.Metadata.Namespace).List(k8sutil.ClusterListOpt(c.cluster.Metadata.Name))
	if err != nil {
		return nil, nil, fmt.Errorf("failed to list running pods: %v", err)
	}

	for i := range podList.Items {
		pod := &podList.Items[i]
		if len(pod.OwnerReferences) < 1 {
			c.logger.Warningf("pollPods: ignore pod %v: no owner", pod.Name)
			continue
		}
		if pod.OwnerReferences[0].UID != c.cluster.Metadata.UID {
			c.logger.Warningf("pollPods: ignore pod %v: owner (%v) is not %v",
				pod.Name, pod.OwnerReferences[0].UID, c.cluster.Metadata.UID)
			continue
		}
		switch pod.Status.Phase {
		case v1.PodRunning:
			running = append(running, pod)
		case v1.PodPending:
			pending = append(pending, pod)
		}
	}

	return running, pending, nil
}

func (c *Cluster) updateMemberStatus(containers []*rancher.Container) {
	var ready, unready []*rancher.Container
	for _, container := range containers {
		// TODO: Change to URL struct for TLS integration
		url := fmt.Sprintf("http://%s:2379", container.PrimaryIpAddress)
		healthy, err := etcdutil.CheckHealth(url)
		if err != nil {
			c.logger.Warningf("health check of etcd member (%s) failed: %v", url, err)
		}
		if healthy {
			ready = append(ready, container)
		} else {
			unready = append(unready, container)
		}
	}
	c.status.Members.Ready = ranchutil.GetContainerNames(ready)
	c.status.Members.Unready = ranchutil.GetContainerNames(unready)
}

func (c *Cluster) updateTPRStatus() error {
	if reflect.DeepEqual(c.cluster.Status, c.status) {
		return nil
	}

	newCluster := c.cluster
	newCluster.Status = c.status
	newCluster, err := ranchutil.UpdateClusterTPRObject(c.config.Client, c.cluster.Metadata.Namespace, newCluster)
	if err != nil {
		return err
	}

	c.cluster = newCluster

	return nil
}

func (c *Cluster) updateLocalBackupStatus() error {
	if c.bm == nil {
		return nil
	}

	bs, err := c.bm.getStatus()
	if err != nil {
		return err
	}
	c.status.BackupServiceStatus = backupServiceStatusToTPRBackupServiceStatu(bs)

	return nil
}

func (c *Cluster) reportFailedStatus() {
	retryInterval := 5 * time.Second

	f := func() (bool, error) {
		c.status.SetPhase(spec.ClusterPhaseFailed)
		err := c.updateTPRStatus()
		if err == nil || k8sutil.IsKubernetesResourceNotFoundError(err) {
			return true, nil
		}

		if !apierrors.IsConflict(err) {
			c.logger.Warningf("retry report status in %v: fail to update: %v", retryInterval, err)
			return false, nil
		}

		cl, err := k8sutil.GetClusterTPRObject(c.config.KubeCli.CoreV1().RESTClient(), c.cluster.Metadata.Namespace, c.cluster.Metadata.Name)
		if err != nil {
			// Update (PUT) will return conflict even if object is deleted since we have UID set in object.
			// Because it will check UID first and return something like:
			// "Precondition failed: UID in precondition: 0xc42712c0f0, UID in object meta: ".
			if k8sutil.IsKubernetesResourceNotFoundError(err) {
				return true, nil
			}
			c.logger.Warningf("retry report status in %v: fail to get latest version: %v", retryInterval, err)
			return false, nil
		}
		c.cluster = cl
		return false, nil

	}

	retryutil.Retry(retryInterval, math.MaxInt64, f)
}
