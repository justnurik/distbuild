//go:build !solution

package client

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	_ "net/http/pprof"

	"go.uber.org/zap"

	"gitlab.com/justnurik/distbuild/pkg/api"
	"gitlab.com/justnurik/distbuild/pkg/build"
	"gitlab.com/justnurik/distbuild/pkg/filecache"
)

type Client struct {
	l *zap.Logger

	apiEndpoint string
	sourceDir   string
}

func NewClient(
	l *zap.Logger,
	apiEndpoint string,
	sourceDir string,
) *Client {
	return &Client{
		l:           l,
		apiEndpoint: apiEndpoint,
		sourceDir:   sourceDir,
	}
}

type BuildListener interface {
	OnJobStdout(jobID build.ID, stdout []byte) error
	OnJobStderr(jobID build.ID, stderr []byte) error

	OnJobFinished(jobID build.ID) error
	OnJobFailed(jobID build.ID, code int, err string) error
}

func (c *Client) Build(ctx context.Context, graph build.Graph, lsn BuildListener) error {
	c.l.Info("build new started")

	buildClient := api.NewBuildClient(c.l, c.apiEndpoint)
	fileCacheClient := filecache.NewClient(c.l, c.apiEndpoint)

	started, statusReader, err := buildClient.StartBuild(ctx, &api.BuildRequest{Graph: graph})
	if err != nil {
		c.l.Error("failed to start build", zap.Error(err))
		return fmt.Errorf("start build: %w", err)
	}
	defer func() { _ = statusReader.Close() }()

	logger := c.l.With(zap.String("build_id", started.ID.String()))

	logger.Info("build started",
		zap.Int("missing_files", len(started.MissingFiles)))

	errs := make(chan error, len(started.MissingFiles))
	for _, fileID := range started.MissingFiles {
		filePath, ok := graph.SourceFiles[fileID]
		if !ok {
			err := fmt.Errorf("file %s not found in source files", fileID)
			logger.Error("file not found",
				zap.String("file_id", fileID.String()),
				zap.Error(err))
			return err
		}

		fullPath := filepath.Join(c.sourceDir, filePath)

		fileInfo, err := os.Stat(fullPath)
		if err != nil {
			logger.Error("failed to stat file",
				zap.String("path", fullPath),
				zap.Error(err))
			return fmt.Errorf("stat file %s: %w", fullPath, err)
		}

		if !fileInfo.Mode().IsRegular() {
			logger.Error("file is not regular",
				zap.String("path", fullPath),
				zap.String("mode", fileInfo.Mode().String()))
			return fmt.Errorf("file %s is not a regular file", fullPath)
		}

		go func() {
			if err := fileCacheClient.Upload(ctx, fileID, fullPath); err != nil {
				logger.Error("failed to upload file",
					zap.String("file_id", fileID.String()),
					zap.String("path", fullPath),
					zap.Error(err))
				errs <- fmt.Errorf("upload file %s: %w", fileID, err)
			}

			errs <- nil
		}()
	}

	for range len(started.MissingFiles) {
		if err := <-errs; err != nil {
			return err
		}
	}

	logger.Info("build upload of the missing files has been completed")

	if _, err := buildClient.SignalBuild(ctx, started.ID, &api.SignalRequest{
		UploadDone: &api.UploadDone{},
	}); err != nil {
		logger.Error("failed to signal upload done", zap.Error(err))
		return fmt.Errorf("signal upload done: %w", err)
	}

	logger.Info("build signal end -> start listen build")

	for {
		switch err := listenBuild(ctx, statusReader, lsn, logger); err {
		case io.EOF:
			return nil
		case nil:
			continue
		default:
			return err
		}
	}
}

func listenBuild(ctx context.Context, statusReader api.StatusReader, lsn BuildListener, logger *zap.Logger) error {

	var update *api.StatusUpdate
	var err error
	done := make(chan struct{})

	go func() {
		update, err = statusReader.Next()
		close(done)
	}()

	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-done:
	}

	logger.Debug("field", zap.Any("update", update))

	if err == io.EOF {
		logger.Info("build completed successfully")
		return io.EOF
	}
	if err != nil {
		logger.Error("failed to get build status update", zap.Error(err))
		return fmt.Errorf("status update: %w", err)
	}

	switch {

	case update.JobFinished != nil:
		logger.Info("job finished",
			zap.String("job_id", update.JobFinished.ID.String()),
			zap.Int("exit_code", update.JobFinished.ExitCode))

		if update.JobFinished.Error != nil {
			if err := lsn.OnJobFailed(update.JobFinished.ID, update.JobFinished.ExitCode, *update.JobFinished.Error); err != nil {
				logger.Error("err in BuildListener.OnJobFailed",
					zap.Error(err),
					zap.Any("update", update))
				return fmt.Errorf("err in BuildListener.OnJobFailed: %w", err)
			}
		} else {
			if err := lsn.OnJobFinished(update.JobFinished.ID); err != nil {
				logger.Error("err in BuildListener.OnJobFinished",
					zap.Error(err),
					zap.Any("update", update))
				return fmt.Errorf("err in BuildListener.OnJobFinished: %w", err)
			}
		}

		if err := lsn.OnJobStderr(update.JobFinished.ID, update.JobFinished.Stderr); err != nil {
			logger.Error("err in BuildListener.OnJobStderr",
				zap.Error(err),
				zap.ByteString("stderr", update.JobFinished.Stderr))
			return fmt.Errorf("err in BuildListener.OnJobStderr: %w", err)
		}

		if err := lsn.OnJobStdout(update.JobFinished.ID, update.JobFinished.Stdout); err != nil {
			logger.Error("err in BuildListener.OnJobStdout",
				zap.Error(err),
				zap.ByteString("stdout", update.JobFinished.Stdout))
			return fmt.Errorf("err in BuildListener.OnJobStdout: %w", err)
		}

	case update.BuildFailed != nil:
		logger.Error("build failed", zap.String("error", update.BuildFailed.Error))
		return fmt.Errorf("build failed: %s", update.BuildFailed.Error)

	case update.BuildFinished != nil:
		logger.Info("build finished successfully")
		return nil

	}

	return nil
}
