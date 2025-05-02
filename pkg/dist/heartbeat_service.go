package dist

import (
	"context"

	"gitlab.com/justnurik/distbuild/pkg/api"
	"gitlab.com/justnurik/distbuild/pkg/build"
	"go.uber.org/zap"
)

type heartbeatService struct {
	l *zap.Logger
	*coordinatorCore
}

func NewHeartbeatService(l *zap.Logger, core *coordinatorCore) *heartbeatService {
	return &heartbeatService{
		l:               l.With(zap.String("component", "heartbeat_service")),
		coordinatorCore: core,
	}
}

func (h *heartbeatService) Heartbeat(ctx context.Context, req *api.HeartbeatRequest) (*api.HeartbeatResponse, error) {
	h.sched.RegisterWorker(req.WorkerID)

	// read worker request

	uniq := make(map[build.ID]struct{})
	h.l.Debug("read worker request", zap.Any("request", req))

	for _, job := range req.FinishedJob {
		exist := h.sched.OnJobComplete(req.WorkerID, job.ID, &job)
		if !exist {
			panic("non schedule job finished")
		}

		uniq[job.ID] = struct{}{}
	}

	for _, jobID := range req.AddedArtifacts {
		if _, exist := uniq[jobID]; !exist {
			h.sched.OnJobComplete(req.WorkerID, jobID, nil)
		}
	}

	// write worker responce

	responce := &api.HeartbeatResponse{
		JobsToRun: make(map[build.ID]api.JobSpec),
	}

	pending := h.sched.PickJob(ctx, req.WorkerID)
	if pending == nil {
		return nil, ctx.Err()
	}
	responce.JobsToRun[pending.Job.ID] = *pending.Job
	req.FreeSlots--

	for range req.FreeSlots {
		for range 4 { // 4 попыток
			pending, ok := h.sched.TryPickJob(ctx, req.WorkerID)
			if ok {
				if pending == nil {
					return nil, ctx.Err()
				}
				responce.JobsToRun[pending.Job.ID] = *pending.Job
				break
			}

		}
	}

	h.l.Debug("write worker responce", zap.Any("responce", responce))

	return responce, nil
}
