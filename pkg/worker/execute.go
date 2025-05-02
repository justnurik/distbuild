package worker

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"
	"os/exec"

	"gitlab.com/justnurik/distbuild/pkg/api"
	"gitlab.com/justnurik/distbuild/pkg/build"
	"go.uber.org/zap"
)

func (w *Worker) executeJob(ctx context.Context, job *api.JobSpec) (jobRes api.JobResult, err error) {
	jobRes.ID = job.ID
	sourceDir := os.TempDir()

	logger := w.log.With(zap.String("job_id", job.ID.String()))

	for fileID, filePath := range job.SourceFiles {
		if err := linkFiles(w.fileCache, sourceDir, fileID, filePath); err != nil {
			logger.Error("couldn't copy all the necessary files to run the command",
				zap.Error(err),
				zap.String("file_id", fileID.String()),
				zap.String("file_path", filePath))
			return jobRes, fmt.Errorf("couldn't copy all the necessary files to run the command: %w", err)
		}
	}

	outputDir, commit, abort, err := w.artifacts.Create(job.ID)
	if err != nil {
		logger.Error("failed to create an artifact", zap.Error(err))
		return jobRes, fmt.Errorf("failed to create an artifact: %w", err)
	}

	jobContext := build.JobContext{
		SourceDir: sourceDir,
		OutputDir: outputDir,
		Deps:      make(map[build.ID]string),
	}

	unlocks := make([]func(), 0)

	for _, depJobID := range job.Deps {
		dir, unlock, err := w.artifacts.Get(depJobID)
		if err != nil {
			panic("top sort does not worlk")
		}
		unlocks = append(unlocks, unlock)

		jobContext.Deps[depJobID] = dir
	}

	for i := range job.Cmds {
		cmd, _ := job.Cmds[i].Render(jobContext)

		stdout := bytes.Buffer{}
		stderr := bytes.Buffer{}

		jobRes.ExitCode, err = executeCommand(ctx, cmd, &stdout, &stderr)
		if err != nil {

			logger.Error("failed job",
				zap.Error(err), zap.Int("exit_code", jobRes.ExitCode))

			errMsg := err.Error()
			jobRes.Error = &errMsg

			if err := abort(); err != nil {
				logger.Error("failed abort", zap.Error(err))
				return jobRes, fmt.Errorf("failed abort: %w", err)
			}

			return jobRes, fmt.Errorf("failed job: %w", err)
		}

		jobRes.Stderr = append(jobRes.Stderr, stderr.Bytes()...)
		jobRes.Stdout = append(jobRes.Stdout, stdout.Bytes()...)
	}

	for i := range unlocks {
		unlocks[i]()
	}

	if err := commit(); err != nil {
		logger.Error("failed commit",
			zap.Error(err), zap.Any("job_result", jobRes))
		return jobRes, fmt.Errorf("failed commit: %w", err)
	}
	return jobRes, nil
}

func executeCommand(ctx context.Context, cmd *build.Cmd, stdout, stderr io.Writer) (int, error) {
	if len(cmd.Exec) > 0 {
		execCommand := exec.CommandContext(ctx, cmd.Exec[0], cmd.Exec[1:]...)

		execCommand.Dir = cmd.WorkingDirectory
		execCommand.Env = cmd.Environ

		execCommand.Stdout = stdout
		execCommand.Stderr = stderr

		return execCommand.ProcessState.ExitCode(), execCommand.Run()
	}

	if len(cmd.CatOutput) > 0 {
		f, err := os.Create(cmd.CatOutput)
		if err == nil {
			_, err = f.WriteString(cmd.CatTemplate)
		}

		return 0, err
	}

	return 0, nil
}
