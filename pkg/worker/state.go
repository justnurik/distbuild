package worker

import (
	"context"
	"runtime"

	"gitlab.com/justnurik/distbuild/pkg/api"
	"gitlab.com/justnurik/distbuild/pkg/build"
)

type workerState struct {
	mu chan struct{}

	AddedArtifacts []build.ID
	FinishedJob    []api.JobResult
	FreeSlots      int
}

func newWorkerState() *workerState {
	w := &workerState{
		mu: make(chan struct{}, 1),

		AddedArtifacts: make([]build.ID, 0),
		FinishedJob:    make([]api.JobResult, 0),

		FreeSlots: runtime.NumCPU(),
	}
	return w
}

func (w *workerState) pull(workerID api.WorkerID) *api.HeartbeatRequest {
	request := &api.HeartbeatRequest{
		WorkerID:       workerID,
		FreeSlots:      w.FreeSlots,
		FinishedJob:    w.FinishedJob,
		AddedArtifacts: w.AddedArtifacts,
	}
	w.AddedArtifacts = make([]build.ID, 0)
	w.FinishedJob = make([]api.JobResult, 0)

	return request
}

func (w *workerState) addArtifacts(ctx context.Context, artifacts []build.ID) {
	select {
	case w.mu <- struct{}{}:
		defer func() { <-w.mu }()
	case <-ctx.Done():
		return
	}

	w.AddedArtifacts = append(w.AddedArtifacts, artifacts...)
}

func (w *workerState) addJobResult(ctx context.Context, jobRes *api.JobResult) {
	select {
	case w.mu <- struct{}{}:
		defer func() { <-w.mu }()
	case <-ctx.Done():
		return
	}

	w.FinishedJob = append(w.FinishedJob, *jobRes)
}
