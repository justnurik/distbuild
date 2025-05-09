package filecache_test

import (
	"errors"
	"os"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/justnurik/distbuild/pkg/build"
	"gitlab.com/justnurik/distbuild/pkg/filecache"
)

type testCache struct {
	*filecache.Cache
	tmpDir string
}

func newCache(t *testing.T) *testCache {
	tmpDir, err := os.MkdirTemp("", "filecache")
	require.NoError(t, err)

	c, err := filecache.New(tmpDir)
	require.NoError(t, err)

	cc := &testCache{Cache: c, tmpDir: tmpDir}
	t.Cleanup(cc.cleanup)
	return cc
}

func (c *testCache) cleanup() {
	_ = os.Remove(c.tmpDir)
}

func TestFileCache(t *testing.T) {
	cache := newCache(t)

	_, abort, err := cache.Write(build.ID{01})
	require.NoError(t, err)
	require.NoError(t, abort())

	_, _, err = cache.Get(build.ID{01})
	require.Truef(t, errors.Is(err, filecache.ErrNotFound), "%v", err)

	f, _, err := cache.Write(build.ID{02})
	require.NoError(t, err)

	_, err = f.Write([]byte("foo bar"))
	require.NoError(t, err)
	require.NoError(t, f.Close())

	path, unlock, err := cache.Get(build.ID{02})
	require.NoError(t, err)
	defer unlock()

	content, err := os.ReadFile(path)
	require.NoError(t, err)
	require.Equal(t, []byte("foo bar"), content)
}
