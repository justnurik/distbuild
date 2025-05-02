package scheduler

import (
	"context"
	"sync"
	"testing"
	"time"

	_ "net/http/pprof"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"gitlab.com/justnurik/distbuild/pkg/api"
	"gitlab.com/justnurik/distbuild/pkg/build"
)

func TestScheduler_BasicFlow(t *testing.T) {
	logger := zaptest.NewLogger(t)
	s := NewScheduler(logger, Config{}, time.After)

	jobSpec := &api.JobSpec{
		Job: build.Job{ID: build.ID{}},
	}
	workerID := api.WorkerID("worker-1")

	pendingJob := s.ScheduleJob(jobSpec)
	require.NotNil(t, pendingJob)

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()

	pickedJob, ok := s.TryPickJob(ctx, workerID)
	require.True(t, ok)
	require.Equal(t, jobSpec.ID, pickedJob.Job.ID)

	result := &api.JobResult{}
	wasKnown := s.OnJobComplete(workerID, jobSpec.ID, result)
	assert.True(t, wasKnown)

	select {
	case <-pendingJob.Finished:
		assert.Equal(t, result, pendingJob.Result)
	case <-time.After(time.Second):
		t.Fatal("timeout waiting for job completion")
	}
}

func TestScheduler_DuplicateSchedule(t *testing.T) {
	logger := zaptest.NewLogger(t)
	s := NewScheduler(logger, Config{}, time.After)

	jobSpec := &api.JobSpec{
		Job: build.Job{ID: build.ID{}},
	}

	pending1 := s.ScheduleJob(jobSpec)
	pending2 := s.ScheduleJob(jobSpec)

	assert.Equal(t, pending1, pending2)
}

func TestScheduler_CompleteBeforeSchedule(t *testing.T) {
	logger := zaptest.NewLogger(t)
	s := NewScheduler(logger, Config{}, time.After)

	jobID := build.ID{}
	workerID := api.WorkerID("worker-1")
	result := &api.JobResult{}

	wasKnown := s.OnJobComplete(workerID, jobID, result)
	assert.False(t, wasKnown)

	jobSpec := &api.JobSpec{
		Job: build.Job{ID: jobID},
	}
	pendingJob := s.ScheduleJob(jobSpec)

	select {
	case <-pendingJob.Finished:
		assert.Equal(t, result, pendingJob.Result)
	default:
		t.Fatal("job should be already completed")
	}
}

func TestScheduler_LocateArtifact(t *testing.T) {
	logger := zaptest.NewLogger(t)
	s := NewScheduler(logger, Config{}, time.After)

	jobID := build.ID{0}
	workerID := api.WorkerID("worker-1")
	result := &api.JobResult{}

	s.OnJobComplete(workerID, jobID, result)

	foundWorker, ok := s.LocateArtifact(jobID)
	assert.True(t, ok)
	assert.Equal(t, workerID, foundWorker)

	_, ok = s.LocateArtifact(build.ID{1})
	assert.False(t, ok)
}

func TestScheduler_Stop(t *testing.T) {
	logger := zaptest.NewLogger(t)
	s := NewScheduler(logger, Config{}, time.After)

	s.Stop()

	assert.Panics(t, func() {
		s.ScheduleJob(&api.JobSpec{Job: build.Job{ID: build.ID{}}})
	})

	assert.Panics(t, func() {
		s.PickJob(context.Background(), api.WorkerID("worker-1"))
	})

	assert.Panics(t, func() {
		s.OnJobComplete(api.WorkerID("worker-1"), build.ID{}, nil)
	})

	assert.Panics(t, func() {
		s.LocateArtifact(build.ID{})
	})
}

func TestScheduler_ConcurrentAccess(t *testing.T) {
	var wg sync.WaitGroup

	logger := zaptest.NewLogger(t)
	s := NewScheduler(logger, Config{}, time.After)

	const workers = 10
	const jobsPerWorker = 1000

	wg.Add(workers * 2)

	idMat := make([][]build.ID, workers)
	for i := range idMat {
		idMat[i] = make([]build.ID, jobsPerWorker)
	}

	for i := range workers {
		go func(workerNum int) {
			defer wg.Done()
			for j := range jobsPerWorker {

				jobID := build.NewID()
				idMat[i][j] = jobID

				s.ScheduleJob(&api.JobSpec{Job: build.Job{ID: jobID}})
			}
		}(i)
	}

	for i := range workers {
		go func(workerNum int) {
			defer wg.Done()
			workerID := api.WorkerID("worker-" + string(rune(workerNum)))
			for j := 0; j < jobsPerWorker; j++ {
				ctx, cancel := context.WithTimeout(context.Background(), time.Second)
				job := s.PickJob(ctx, workerID)
				cancel()
				if job != nil {
					s.OnJobComplete(workerID, job.Job.ID, &api.JobResult{})
				}
			}
		}(i)
	}

	wg.Wait()

	for i := range workers {
		for j := range jobsPerWorker {

			jobID := idMat[i][j]

			_, ok := s.LocateArtifact(jobID)
			assert.True(t, ok, "artifact for job %s not found", jobID)
		}
	}
}
