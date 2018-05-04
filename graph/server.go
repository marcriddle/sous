package graph

import (
	"github.com/opentable/sous/config"
	sous "github.com/opentable/sous/lib"
	"github.com/opentable/sous/server"
	"github.com/samsalisbury/semv"
)

func newServerComponentLocator(
	ls LogSink,
	cfg LocalSousConfig,
	ins sous.Inserter,
	sm *ServerStateManager,
	rf *sous.ResolveFilter,
	ar *sous.AutoResolver,
	v semv.Version,
	qs *sous.R11nQueueSet,
) server.ComponentLocator {
	cm := sous.MakeClusterManager(sm.StateManager)
	dm := sous.MakeDeploymentManager(sm.StateManager)
	return server.ComponentLocator{

		LogSink:           ls.LogSink,
		Config:            cfg.Config,
		Inserter:          ins,
		StateManager:      sm.StateManager,
		ClusterManager:    cm,
		DeploymentManager: dm,
		ResolveFilter:     rf,
		AutoResolver:      ar,
		Version:           v,
		QueueSet:          qs,
	}

}

func newClusterManager(sm *ServerStateManager) sous.ClusterManager {
	return sous.MakeClusterManager(sm.StateManager)
}

func newSousStateManager(sm *ServerStateManager) sous.StateManager {
	return sm.StateManager
}

func newConfig(c LocalSousConfig) *config.Config {
	return c.Config
}

// NewR11nQueueSet returns a new queue set configured to start processing r11ns
// immediately.
func NewR11nQueueSet(d sous.Deployer, r sous.Registry, rf *sous.ResolveFilter, sm *ServerStateManager) *sous.R11nQueueSet {
	sr := sm.StateManager
	return sous.NewR11nQueueSet(sous.R11nQueueStartWithHandler(
		func(qr *sous.QueuedR11n) sous.DiffResolution {
			qr.Rectification.Begin(d, r, rf, sr)
			return qr.Rectification.Wait()
		}))
}

func newQueueSet(qs *sous.R11nQueueSet) sous.QueueSet {
	return qs
}
