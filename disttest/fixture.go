package disttest

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
	"go.uber.org/goleak"

	"gitlab.com/justnurik/distbuild/pkg/api"
	"gitlab.com/justnurik/distbuild/pkg/artifact"
	"gitlab.com/justnurik/distbuild/pkg/client"
	"gitlab.com/justnurik/distbuild/pkg/dist"
	"gitlab.com/justnurik/distbuild/pkg/filecache"
	"gitlab.com/justnurik/distbuild/pkg/worker"
	"gitlab.com/slon/shad-go/tools/testtool"

	"go.uber.org/zap"
)

type env struct {
	RootDir string
	Logger  *zap.Logger

	Ctx context.Context

	Client      *client.Client
	Coordinator *dist.Coordinator
	Workers     []*worker.Worker
	WorkerCache []*artifact.Cache

	HTTP *http.Server
}

const (
	logToStderr = true
)

type Config struct {
	WorkerCount int
}

func newEnv(t *testing.T, config *Config) (e *env) {
	t.Cleanup(func() {
		goleak.VerifyNone(t)
	})

	cwd, err := os.Getwd()
	require.NoError(t, err)

	absCWD, err := filepath.Abs(cwd)
	require.NoError(t, err)

	rootDir := filepath.Join(absCWD, "workdir", t.Name())
	require.NoError(t, os.RemoveAll(rootDir))

	if err = os.MkdirAll(rootDir, 0777); err != nil {
		if errors.Is(err, os.ErrPermission) {
			rootDir, err = os.MkdirTemp("", "")
			require.NoError(t, err)
		} else {
			require.NoError(t, err)
		}
	}

	env := &env{
		RootDir: rootDir,
	}

	cfg := zap.NewDevelopmentConfig()

	if runtime.GOOS == "windows" {
		cfg.OutputPaths = []string{filepath.Join("winfile://", env.RootDir, "test.log")}
		err = zap.RegisterSink("winfile", newWinFileSink)
		require.NoError(t, err)
	} else {
		cfg.OutputPaths = []string{filepath.Join(env.RootDir, "test.log")}
	}

	if logToStderr {
		cfg.OutputPaths = append(cfg.OutputPaths, "stderr")
	}

	env.Logger, err = cfg.Build()
	require.NoError(t, err)

	t.Cleanup(func() {
		_ = env.Logger.Sync()
	})

	t.Helper()
	t.Logf("test is running inside %s; see test.log file for more info", filepath.Join("workdir", t.Name()))

	port, err := testtool.GetFreePort()
	require.NoError(t, err)
	addr := "127.0.0.1:" + port
	coordinatorEndpoint := "http://" + addr + "/coordinator"

	var cancelRootContext func()
	env.Ctx, cancelRootContext = context.WithCancel(context.Background())
	t.Cleanup(cancelRootContext)

	env.Client = client.NewClient(
		env.Logger.Named("client"),
		coordinatorEndpoint,
		filepath.Join(absCWD, "testdata", t.Name()))

	coordinatorCache, err := filecache.New(filepath.Join(env.RootDir, "coordinator", "filecache"))
	require.NoError(t, err)

	env.Coordinator = dist.NewCoordinator(
		env.Logger.Named("coordinator"),
		coordinatorCache,
	)
	t.Cleanup(env.Coordinator.Stop)

	router := http.NewServeMux()
	router.Handle("/coordinator/", http.StripPrefix("/coordinator", env.Coordinator))

	for i := 0; i < config.WorkerCount; i++ {
		workerName := fmt.Sprintf("worker%d", i)
		workerDir := filepath.Join(env.RootDir, workerName)

		var fileCache *filecache.Cache
		fileCache, err = filecache.New(filepath.Join(workerDir, "filecache"))
		require.NoError(t, err)

		var artifacts *artifact.Cache
		artifacts, err = artifact.NewCache(filepath.Join(workerDir, "artifacts"))
		require.NoError(t, err)

		workerPrefix := fmt.Sprintf("/worker/%d", i)
		workerID := api.WorkerID("http://" + addr + workerPrefix)

		w := worker.New(
			workerID,
			coordinatorEndpoint,
			env.Logger.Named(workerName),
			fileCache,
			artifacts,
		)

		env.Workers = append(env.Workers, w)
		env.WorkerCache = append(env.WorkerCache, artifacts)

		router.Handle(workerPrefix+"/", http.StripPrefix(workerPrefix, w))
	}

	env.HTTP = &http.Server{
		Addr:    addr,
		Handler: router,
	}

	lsn, err := net.Listen("tcp", env.HTTP.Addr)
	require.NoError(t, err)

	go func() {
		err := env.HTTP.Serve(lsn)
		if err != http.ErrServerClosed {
			env.Logger.Fatal("http server stopped", zap.Error(err))
		}
	}()

	t.Cleanup(func() {
		cancelRootContext()
		_ = env.HTTP.Shutdown(context.Background())
	})

	for _, w := range env.Workers {
		go func(w *worker.Worker) {
			err := w.Run(env.Ctx)
			if errors.Is(err, context.Canceled) {
				return
			}

			env.Logger.Fatal("worker stopped", zap.Error(err))
		}(w)
	}

	go func() {
		select {
		case <-time.After(time.Second * 10):
			panic("test hang")
		case <-env.Ctx.Done():
			return
		}
	}()

	return env
}

func newWinFileSink(u *url.URL) (zap.Sink, error) {
	if len(u.Opaque) > 0 {
		// Remove leading slash left by url.Parse()
		return os.OpenFile(u.Opaque[1:], os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
	}
	// if url.URL is empty, don't panic slice index error
	return os.OpenFile(u.Opaque, os.O_WRONLY|os.O_APPEND|os.O_CREATE, 0644)
}
