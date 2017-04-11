package k8s

import (
  "github.com/coreos/etcd-operator/pkg/backup/s3/s3config"
  "github.com/coreos/etcd-operator/pkg/k8s/controller"
  "github.com/coreos/etcd-operator/pkg/k8s/garbagecollection"
  "github.com/coreos/etcd-operator/pkg/k8s/k8sutil"
  "github.com/coreos/etcd-operator/pkg/spec"

  "github.com/Sirupsen/logrus"
  "github.com/urfave/cli"
)

func NewKubernetesOperator(c *cli.Context) *spec.Operator {
  // Workaround for watching TPR resource.
  restCfg, err := k8sutil.InClusterConfig()
  if err != nil {
    panic(err)
  }
  controller.MasterHost = restCfg.Host
  restcli, err := k8sutil.NewTPRClient()
  if err != nil {
    panic(err)
  }
  controller.KubeHttpCli = restcli.Client

  cfg := newK8sControllerConfig(c)
  if err := cfg.Validate(); err != nil {
    logrus.Fatalf("invalid operator config: %v", err)
  }

  return &spec.Operator{
    Controller: controller.New(cfg),
    GC:         garbagecollection.New(cfg.KubeCli, cfg.Namespace),
    GCPeriod:   c.Duration("gc-interval"),
    OptIn:      c.Bool("analytics"),
  }
}

func newK8sControllerConfig(c *cli.Context) controller.Config {
  kubecli := k8sutil.MustNewKubeClient()

  pod, err := k8sutil.GetPodByName(kubecli, c.String("namespace"), c.String("name"))
  if err != nil {
    logrus.Fatalf("failed to get my pod: %v", err)
  }
  if pod.Spec.ServiceAccountName == "" {
    logrus.Fatalf("my pod has no service account!")
  }

  return controller.Config{
    Namespace:      c.String("namespace"),
    ServiceAccount: pod.Spec.ServiceAccountName,
    PVProvisioner:  c.String("pv-provisioner"),
    S3Context: s3config.S3Context{
      AWSSecret: c.String("backup-aws-secret"),
      AWSConfig: c.String("backup-aws-config"),
      S3Bucket:  c.String("backup-s3-bucket"),
    },
    KubeCli: kubecli,
  }
}
