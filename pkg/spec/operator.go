package spec

import (
	"os"
	"os/signal"
	"time"

	"github.com/coreos/etcd-operator/pkg/analytics"

	"github.com/Sirupsen/logrus"
)

type Operator struct {
	Controller Controller
	GC         GarbageCollection
	GCPeriod   time.Duration
	OptIn      bool
}

// Controller watches for created/modified/deleted clusters
type Controller interface {
	Run() error
}

// GarbageCollection deletes objects that no longer belong to any cluster.
type GarbageCollection interface {
	FullyCollect() error
}

// TODO obey the stop channel
func (o *Operator) Run(stop <-chan struct{}) {
	ch := make(chan os.Signal, 1)
	signal.Notify(ch)
	go func() {
		logrus.Infof("received signal: %v", <-ch)
		os.Exit(1)
	}()

	if o.OptIn {
		analytics.Enable()
	}

	analytics.OperatorStarted()
	go o.periodicallyGC()
	o.Controller.Run()
}

func (o *Operator) periodicallyGC() {
	t := time.NewTicker(o.GCPeriod)
	for range t.C {
		if err := o.GC.FullyCollect(); err != nil {
			logrus.Warningf("failed to cleanup resources: %v", err)
		}
	}
}
