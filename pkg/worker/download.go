package worker

import (
	"context"
	"errors"
	"fmt"

	"gitlab.com/justnurik/distbuild/pkg/api"
	"gitlab.com/justnurik/distbuild/pkg/artifact"
	"gitlab.com/justnurik/distbuild/pkg/build"
	"gitlab.com/justnurik/distbuild/pkg/filecache"
	"go.uber.org/zap"
)

func (w *Worker) downloadFiles(ctx context.Context, job *api.JobSpec) error {
	for fileID, filePath := range job.SourceFiles {
		_, unlock, err := w.fileCache.Get(fileID)

		switch {
		case errors.Is(err, filecache.ErrNotFound):

		case err == nil:
			unlock()
			continue

		default:
			w.log.Error("unexpected error in `fileCache.Cache.Get`",
				zap.Error(err),
				zap.String("file_id", fileID.String()),
				zap.Any("file_path", filePath))
			return fmt.Errorf("unexpected error in `fileCache.Cache.Get`: %w", err)
		}

		if err := w.fileCacheClient.Download(ctx, w.fileCache, fileID); err != nil {
			w.log.Error("couldn't download the files needed for the build from the coordinator",
				zap.Error(err),
				zap.String("file_id", fileID.String()),
				zap.String("file_path", filePath))
			return fmt.Errorf("couldn't download the files needed for the build from the coordinator: %w", err)
		}

	}

	return nil
}

func (w *Worker) downloadArtifacts(ctx context.Context, job *api.JobSpec) error {
	addedArtifacts := make([]build.ID, 0)
	defer w.state.addArtifacts(ctx, addedArtifacts)

	//* artifacts
	for artifactsID, workerID := range job.Artifacts {
		_, unlock, err := w.artifacts.Get(artifactsID)

		switch {
		case errors.Is(err, artifact.ErrNotFound):

		case err == nil:
			unlock()
			continue

		default:
			w.log.Error("unexpected error in `artifact.Cache.Get`",
				zap.Error(err),
				zap.String("artifacts_id", artifactsID.String()),
				zap.String("worker_id", workerID.String()))
			return fmt.Errorf("unexpected error in `artifact.Cache.Get`: %w", err)
		}

		if err := artifact.Download(ctx, workerID.String(), w.artifacts, artifactsID); err != nil {
			w.log.Error("couldn't download artifact from worker",
				zap.Error(err),
				zap.String("artifact_id", artifactsID.String()))
			return fmt.Errorf("couldn't download artifact from worker: %w", err)
		}

		addedArtifacts = append(addedArtifacts, artifactsID)
	}

	return nil
}
