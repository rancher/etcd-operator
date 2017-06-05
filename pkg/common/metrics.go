package common

import "github.com/prometheus/client_golang/prometheus"

var (
	ClustersTotal = prometheus.NewGauge(prometheus.GaugeOpts{
		Namespace: "etcd_operator",
		Subsystem: "controller",
		Name:      "clusters",
		Help:      "Total number of clusters managed by the controller",
	})

	ClustersCreated = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd_operator",
		Subsystem: "controller",
		Name:      "clusters_created",
		Help:      "Total number of clusters created",
	})

	ClustersDeleted = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd_operator",
		Subsystem: "controller",
		Name:      "clusters_deleted",
		Help:      "Total number of clusters deleted",
	})

	ClustersModified = prometheus.NewCounter(prometheus.CounterOpts{
		Namespace: "etcd_operator",
		Subsystem: "controller",
		Name:      "clusters_modified",
		Help:      "Total number of clusters modified",
	})
)

func init() {
	prometheus.MustRegister(ClustersTotal)
	prometheus.MustRegister(ClustersCreated)
	prometheus.MustRegister(ClustersDeleted)
	prometheus.MustRegister(ClustersModified)
}
