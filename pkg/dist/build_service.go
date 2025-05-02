package dist

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"

	"gitlab.com/justnurik/distbuild/pkg/api"
	"gitlab.com/justnurik/distbuild/pkg/build"
	"gitlab.com/justnurik/distbuild/pkg/filecache"
	"gitlab.com/justnurik/distbuild/pkg/scheduler"
	"go.uber.org/zap"
)

type buildService struct {
	l *zap.Logger
	*coordinatorCore
}

func NewBuildService(l *zap.Logger, core *coordinatorCore) *buildService {
	return &buildService{
		l:               l.With(zap.String("component", "build_service")),
		coordinatorCore: core,
	}
}

func (c *buildService) StartBuild(ctx context.Context, request *api.BuildRequest, w api.StatusWriter) error {
	missingFiles := make([]build.ID, 0)
	fileNameToID := make(map[string]build.ID)

	// todo: randevu [client -----<download/upload files>-----> worker]
	for fileID, filePath := range request.Graph.SourceFiles {
		fileNameToID[filePath] = fileID
		missingFiles = append(missingFiles, fileID)
	}

	started := &api.BuildStarted{
		ID:           build.NewID(),
		MissingFiles: missingFiles,
	}

	if err := w.Started(started); err != nil {
		c.l.Error("error sending the first message by the coordinator",
			zap.Error(err),
			zap.Any("msg", started))
		return fmt.Errorf("error sending the first message by the coordinator: %w", err)
	}

	jobs := build.TopSort(request.Graph.Jobs)

	sourceFiles := make([]map[build.ID]string, len(jobs))

	for i, job := range jobs {
		sourceFiles[i] = make(map[build.ID]string)

		for _, input := range job.Inputs {
			fileID := fileNameToID[input]
			sourceFiles[i][fileID] = input
		}
	}

	c.hb.Happen(started.ID, func() {
		c.buildSourceFiles.Store(started.ID, sourceFiles)
		c.buildGraph.Store(started.ID, jobs)
		c.buildStatusWriter.Store(started.ID, w)
	})

	return nil
}

func (c *buildService) SignalBuild(ctx context.Context, buildID build.ID, signal *api.SignalRequest) (*api.SignalResponse, error) {
	c.hb.Before(buildID)

	if signal.UploadDone == nil {
		return &api.SignalResponse{}, nil
	}

	sourceFiles, exist1 := c.buildSourceFiles.Load(buildID)
	jobs, exist2 := c.buildGraph.Load(buildID)
	sw, exist3 := c.buildStatusWriter.Load(buildID)

	if !exist3 || !exist2 || !exist1 {
		panic("concurrency.HappenceBeforeMachine does not work: require call the function `StartBuild` before `SignalBuild`")
	}

	jobPending := make(map[build.ID]*scheduler.PendingJob)
	finishedJobCount := atomic.Uint64{}

	var wg sync.WaitGroup
	errs := make([]error, 0, len(jobs))

	wg.Add(len(jobs))

	for i, job := range jobs {
		artifacts := make(map[build.ID]api.WorkerID)

		for _, dep := range job.Deps {
			depPending, exist := jobPending[dep]
			if !exist {
				panic("top sort does not work")
			}

			select {
			case <-ctx.Done():
				return nil, ctx.Err()
			case <-depPending.Finished: // wait worker
			}

			depWorkerID, exist := c.sched.LocateArtifact(dep)
			if !exist {
				panic("top sort does not work")
			}

			artifacts[dep] = depWorkerID
		}

		pending := c.sched.ScheduleJob(&api.JobSpec{
			SourceFiles: sourceFiles[i],
			Artifacts:   artifacts,
			Job:         job})
		jobPending[job.ID] = pending

		go func() {
			defer wg.Done()

			select {
			case <-ctx.Done():
				errs = append(errs, ctx.Err())
				return
			case <-pending.Finished:
				finishedJobCount.Add(1)
			}

			update := api.StatusUpdate{JobFinished: pending.Result}
			if pending.Result.Error != nil {
				update.BuildFailed = &api.BuildFailed{
					Error: *pending.Result.Error,
				}
			}
			if finishedJobCount.CompareAndSwap(uint64(len(jobs)), 0) {
				update.BuildFinished = &api.BuildFinished{}
			}
			if err := sw.Updated(&update); err != nil {
				c.l.Error("error when trying to update the build status",
					zap.Error(err),
					zap.Any("update", update))
				errs = append(errs, fmt.Errorf("error when trying to update the build status: %w", err))
				return
			}

			if err := removeSourceFiles(pending.Job.SourceFiles, c.fileCache, c.l); err != nil {
				c.l.Warn("couldn't delete the sources on the coordinator", zap.Error(err))
			}
		}()
	}

	wg.Wait()

	for _, err := range errs {
		return nil, err
	}

	return &api.SignalResponse{}, ctx.Err()
}

func removeSourceFiles(sourceFiles map[build.ID]string, fileCache *filecache.Cache, logger *zap.Logger) error {
	for fileID := range sourceFiles {
		if err := fileCache.Remove(fileID); err != nil {
			logger.Warn("couldn't delete the file",
				zap.Error(err),
				zap.String("file", sourceFiles[fileID]))
			return fmt.Errorf("couldn't delete the file: %w", err)
		}
	}

	return nil
}
