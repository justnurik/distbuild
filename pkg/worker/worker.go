//go:build !solution

package worker

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"sync"
	"time"

	"go.uber.org/zap"

	"gitlab.com/justnurik/distbuild/pkg/api"
	"gitlab.com/justnurik/distbuild/pkg/artifact"
	"gitlab.com/justnurik/distbuild/pkg/build"
	"gitlab.com/justnurik/distbuild/pkg/concurrency"
	"gitlab.com/justnurik/distbuild/pkg/filecache"
)

type httpAPI struct {
	coordinatorEndpoint string

	// client
	heartbeatClient *api.HeartbeatClient
	fileCacheClient *filecache.Client

	// handler
	mux *http.ServeMux
}

type metaData struct {
	// id
	workerID api.WorkerID

	// state
	state   *workerState
	metrics *workerMetrics
}

type cache struct {
	// file/artifact cache
	fileCache *filecache.Cache
	artifacts *artifact.Cache

	// job result cache
	jobResultCache *concurrency.SyncMap[build.ID, *api.JobResult]
}

type Worker struct {
	log *zap.Logger

	httpAPI
	metaData
	cache
}

func New(
	workerID api.WorkerID,
	coordinatorEndpoint string,
	log *zap.Logger,
	fileCache *filecache.Cache,
	artifacts *artifact.Cache,
) *Worker {
	mux := http.NewServeMux()

	artifactHandler := artifact.NewHandler(log, artifacts)
	artifactHandler.Register(mux)

	// todo: normal
	nums := workerID.String()
	num := nums[len(nums)-2:]
	statFile, _ := os.Create("/home/nurik/work/Nurik/distbuild/stat/" + num)

	return &Worker{
		log: log,

		httpAPI: httpAPI{
			coordinatorEndpoint: coordinatorEndpoint,
			heartbeatClient:     api.NewHeartbeatClient(log, coordinatorEndpoint),
			fileCacheClient:     filecache.NewClient(log, coordinatorEndpoint),
			mux:                 mux,
		},
		metaData: metaData{
			workerID: workerID,
			state:    newWorkerState(),
			metrics:  newWorkerMetrics(statFile, time.Second/10),
		},
		cache: cache{
			fileCache: fileCache,
			artifacts: artifacts,

			jobResultCache: concurrency.NewSyncMap[build.ID, *api.JobResult](0),
		},
	}
}

func (w *Worker) ServeHTTP(rw http.ResponseWriter, r *http.Request) {
	w.mux.ServeHTTP(rw, r)
}

func (w *Worker) Run(ctx context.Context) error {
	defer w.metrics.stop()

	for cycleNum := 0; ; cycleNum++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
			w.log.Info(fmt.Sprintf("start work cycle: %d", cycleNum))
		}

		request := w.state.pull(w.workerID)

		response, err := w.heartbeatClient.Heartbeat(ctx, request)
		if err != nil {
			w.log.Error("couldn't send a `Heartbeat` request to the coordinator",
				zap.Error(err),
				zap.Any("request", request))
			return fmt.Errorf("couldn't send a `Heartbeat` request to the coordinator: %w", err)
		}

		w.log.Info(fmt.Sprintf("schedule: %d", len(response.JobsToRun)))

		if len(response.JobsToRun) == 0 {
			continue
		}

		var wg sync.WaitGroup
		wg.Add(len(response.JobsToRun))

		for jobID, job := range response.JobsToRun {
			w.metrics.scheduleTask()
			go func() {
				defer wg.Done()
				w.runJob(ctx, jobID, &job)
			}()
		}

		wg.Wait()
	}
}

func (w *Worker) runJob(ctx context.Context, jobID build.ID, job *api.JobSpec) {
	w.log.Info("run job", zap.String("job_id", jobID.String()))

	var jobRes = &api.JobResult{
		ID:       jobID,
		Stdout:   nil,
		Stderr:   nil,
		ExitCode: 0,
		Error:    nil,
	}
	defer func() {
		w.metrics.doneTask()
		w.state.addJobResult(ctx, jobRes)
	}()

	if jobResOther, exist := w.jobResultCache.Load(jobID); exist {
		w.log.Debug("cache hit", zap.String("job_id", job.ID.String()))
		jobRes = jobResOther
		return
	}

	w.log.Debug("cache miss", zap.String("job_id", job.ID.String()))

	if err := w.downloadArtifacts(ctx, job); err != nil {
		err := err.Error()
		jobRes.Error = &err
		return
	}
	if err := w.downloadFiles(ctx, job); err != nil {
		err := err.Error()
		jobRes.Error = &err
		return
	}

	res, err := w.executeJob(ctx, job)
	jobRes = &res
	if err != nil {
		w.log.Error("error when executing a job on a worker",
			zap.Error(err),
			zap.String("job_id", jobID.String()),
		)

		err := fmt.Errorf("error when executing a job on a worker: %w", err).Error()
		jobRes.Error = &err
		return
	}

	w.jobResultCache.Store(jobID, jobRes)
}
