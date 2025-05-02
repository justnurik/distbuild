//go:build !solution

package dist

import (
	"net/http"
	"time"

	"go.uber.org/zap"

	"gitlab.com/justnurik/distbuild/pkg/api"
	"gitlab.com/justnurik/distbuild/pkg/build"
	"gitlab.com/justnurik/distbuild/pkg/concurrency"
	"gitlab.com/justnurik/distbuild/pkg/filecache"
	"gitlab.com/justnurik/distbuild/pkg/scheduler"
)

type coordinatorCore struct {
	sched     *scheduler.Scheduler
	fileCache *filecache.Cache

	// buildID ->
	buildGraph        *concurrency.SyncMap[build.ID, []build.Job]
	buildSourceFiles  *concurrency.SyncMap[build.ID, []map[build.ID]string]
	buildStatusWriter *concurrency.SyncMap[build.ID, api.StatusWriter]

	hb *concurrency.HappenceBeforeMachine[build.ID]
}

type Coordinator struct {
	log  *zap.Logger
	mux  *http.ServeMux
	core *coordinatorCore
}

var defaultConfig = scheduler.Config{
	CacheTimeout: time.Millisecond * 10,
	DepsTimeout:  time.Millisecond * 100,
}

func NewCoordinator(
	log *zap.Logger,
	fileCache *filecache.Cache,
) *Coordinator {

	core := &coordinatorCore{
		sched:     scheduler.NewScheduler(log, defaultConfig, time.After),
		fileCache: fileCache,

		buildGraph:        concurrency.NewSyncMap[build.ID, []build.Job](0),
		buildSourceFiles:  concurrency.NewSyncMap[build.ID, []map[build.ID]string](0),
		buildStatusWriter: concurrency.NewSyncMap[build.ID, api.StatusWriter](0),

		hb: concurrency.NewHappenceBeforeMachine[build.ID](),
	}

	c := &Coordinator{
		log:  log,
		mux:  http.NewServeMux(),
		core: core,
	}

	buildHandler := api.NewBuildService(log, NewBuildService(log, core))
	heartbeatHandler := api.NewHeartbeatHandler(log, NewHeartbeatService(log, core))
	fileCacheHandler := filecache.NewHandler(log, fileCache)

	buildHandler.Register(c.mux)
	heartbeatHandler.Register(c.mux)
	fileCacheHandler.Register(c.mux)

	return c
}

func (c *Coordinator) Stop() {
	c.core.sched.Stop()
}

func (c *Coordinator) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mux.ServeHTTP(w, r)
}
