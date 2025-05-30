package filecache_test

import (
	"bytes"
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.uber.org/zap/zaptest"

	"gitlab.com/justnurik/distbuild/pkg/build"
	"gitlab.com/justnurik/distbuild/pkg/filecache"
)

type env struct {
	cache  *testCache
	server *httptest.Server
	client *filecache.Client
}

func newEnv(t *testing.T) *env {
	l := zaptest.NewLogger(t)
	mux := http.NewServeMux()

	cache := newCache(t)

	handler := filecache.NewHandler(l, cache.Cache)
	handler.Register(mux)

	server := httptest.NewServer(mux)
	t.Cleanup(server.Close)

	client := filecache.NewClient(l, server.URL)

	env := &env{
		cache:  cache,
		server: server,
		client: client,
	}

	return env
}

func TestFileUpload(t *testing.T) {
	env := newEnv(t)
	content := bytes.Repeat([]byte("foobar"), 1024*1024)

	tmpFilePath := filepath.Join(env.cache.tmpDir, "foo.txt")
	require.NoError(t, os.WriteFile(tmpFilePath, content, 0666))

	ctx := context.Background()

	t.Run("UploadSingleFile", func(t *testing.T) {
		id := build.ID{0x01}

		require.NoError(t, env.client.Upload(ctx, id, tmpFilePath))

		path, unlock, err := env.cache.Get(id)
		require.NoError(t, err)
		defer unlock()

		actualContent, err := os.ReadFile(path)
		require.NoError(t, err)
		require.Equal(t, content, actualContent)
	})

	t.Run("RepeatedUpload", func(t *testing.T) {
		id := build.ID{0x02}

		require.NoError(t, env.client.Upload(ctx, id, tmpFilePath))
		require.NoError(t, env.client.Upload(ctx, id, tmpFilePath))
	})

	t.Run("ConcurrentUpload", func(t *testing.T) {
		const (
			N = 10
			G = 10
		)

		for i := 0; i < N; i++ {
			var wg sync.WaitGroup
			wg.Add(G)

			id := build.ID{0x03, byte(i)}
			for j := 0; j < G; j++ {
				go func() {
					defer wg.Done()

					assert.NoError(t, env.client.Upload(ctx, id, tmpFilePath))
				}()
			}

			wg.Wait()
		}
	})
}

func TestFileDownload(t *testing.T) {
	env := newEnv(t)

	localCache := newCache(t)

	id := build.ID{0x01}

	w, abort, err := env.cache.Write(id)
	require.NoError(t, err)
	defer func() { _ = abort() }()

	_, err = w.Write([]byte("foobar"))
	require.NoError(t, err)
	require.NoError(t, w.Close())

	ctx := context.Background()
	require.NoError(t, env.client.Download(ctx, localCache.Cache, id))

	path, unlock, err := localCache.Get(id)
	require.NoError(t, err)
	defer unlock()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, []byte("foobar"), content)
}
