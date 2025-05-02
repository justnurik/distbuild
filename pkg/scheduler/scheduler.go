//go:build !solution
// +build !solution

package scheduler

import (
	"context"
	"math/rand"
	"sync"
	"sync/atomic"
	"time"

	"go.uber.org/zap"

	"gitlab.com/justnurik/distbuild/pkg/api"
	"gitlab.com/justnurik/distbuild/pkg/build"
)

type PendingJob struct {
	Job      *api.JobSpec
	Finished chan struct{}
	Result   *api.JobResult

	isCloseFinished atomic.Bool
}

type Config struct {
	CacheTimeout time.Duration
	DepsTimeout  time.Duration
}

type Queue[T any] interface {
	Push(T)
	Pop() <-chan T
}

type Scheduler struct {
	l *zap.Logger

	isStop chan struct{}
	queue  Queue[*PendingJob]

	jobs   map[build.ID]*PendingJob
	jobsMu sync.Mutex

	artifacts   map[build.ID][]api.WorkerID
	artifactsMu sync.RWMutex
}

type workerCache = []api.WorkerID

func NewScheduler(l *zap.Logger, config Config, timeAfter func(d time.Duration) <-chan time.Time) *Scheduler {
	_ = config    // ignore
	_ = timeAfter // ignore

	return &Scheduler{
		l:         l.With(zap.String("component", "scheduler")),
		queue:     newChanQueue[*PendingJob](),
		jobs:      make(map[build.ID]*PendingJob),
		artifacts: make(map[build.ID]workerCache),
		isStop:    make(chan struct{}),
	}
}

func (c *Scheduler) LocateArtifact(id build.ID) (api.WorkerID, bool) {
	c.checkIsStop("call `LocateArtifact` after stop scheduling")

	c.artifactsMu.RLock()
	defer c.artifactsMu.RUnlock()

	workersID, exist := c.artifacts[id]

	defer c.l.Info("locate_artifact",
		zap.Any("workers_id", workersID),
		zap.Bool("exist", exist))

	if len(workersID) == 0 {
		return api.WorkerID(""), false
	}

	workerID := workersID[rand.Intn(len(workersID))]
	return workerID, exist
}

func (c *Scheduler) OnJobComplete(workerID api.WorkerID, jobID build.ID, res *api.JobResult) bool {
	c.checkIsStop("call `OnJobComplete` after stop scheduling")

	c.jobsMu.Lock()

	defer c.l.Info("complete job",
		zap.Any("worker_id", workerID),
		zap.String("job_id", jobID.String()),
		zap.Any("job result", res),
		zap.Any("jobs", c.jobs[jobID]))

	pendingJob, exist := c.jobs[jobID]
	if !exist {
		pendingJob = &PendingJob{
			Job: &api.JobSpec{
				Job: build.Job{ID: jobID},
			},
			Finished: make(chan struct{}),
			Result:   res,
		}

		c.jobs[jobID] = pendingJob
	}
	if pendingJob.isCloseFinished.CompareAndSwap(false, true) {
		pendingJob.Result = res
		close(pendingJob.Finished)
	}
	c.jobsMu.Unlock()

	c.artifactsMu.Lock()

	workersID := c.artifacts[jobID]
	workersID = append(workersID, workerID)
	if len(workersID) == 4+1 {
		workersID = workersID[1:]
	}
	c.artifacts[jobID] = workersID

	c.artifactsMu.Unlock()

	return exist
}

func (c *Scheduler) RegisterWorker(api.WorkerID) {}

func (c *Scheduler) ScheduleJob(job *api.JobSpec) *PendingJob {
	c.checkIsStop("call `ScheduleJob` after stop scheduling")
	c.l.Info("schedule job", zap.Any("job", job))

	c.jobsMu.Lock()
	if p, exist := c.jobs[job.ID]; exist {
		c.jobsMu.Unlock()
		return p
	}

	item := &PendingJob{
		Job:      job,
		Finished: make(chan struct{}),
		Result:   nil,
	}

	c.queue.Push(item)

	c.jobs[job.ID] = item
	c.jobsMu.Unlock()

	return item
}

func (c *Scheduler) PickJob(ctx context.Context, workerID api.WorkerID) *PendingJob {
	c.checkIsStop("call `PickingJob` after stop scheduling")
	defer c.l.Info("pick job", zap.Any("worker_id", workerID))

	select {
	case pendingJob := <-c.queue.Pop():

		return pendingJob
	case <-ctx.Done():
		return nil
	}
}

func (c *Scheduler) TryPickJob(ctx context.Context, workerID api.WorkerID) (pending *PendingJob, ok bool) {
	c.checkIsStop("call `TryPickingJob` after stop scheduling")

	ok = false
	pending = nil

	select {
	case pending = <-c.queue.Pop():
		ok = true
	case <-ctx.Done():
		ok = true
	default:
	}

	c.l.Debug("try pick job", zap.Any("worker_id", workerID), zap.Bool("pick", ok))
	return
}

func (c *Scheduler) checkIsStop(msg string) {
	select {
	case <-c.isStop:
		c.l.Error(msg)
		panic(msg)

	default:
	}
}

func (c *Scheduler) Stop() {
	close(c.isStop)
}
