package artifact_test

import (
	"errors"
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/require"

	"gitlab.com/justnurik/distbuild/pkg/artifact"
	"gitlab.com/justnurik/distbuild/pkg/build"
)

type testCache struct {
	*artifact.Cache
	tmpDir string
}

func (c *testCache) cleanup() {
	_ = os.RemoveAll(c.tmpDir)
}

func newTestCache(t *testing.T) *testCache {
	tmpDir, err := os.MkdirTemp("", "")
	require.NoError(t, err)

	cache, err := artifact.NewCache(tmpDir)
	if err != nil {
		_ = os.RemoveAll(tmpDir)
	}
	require.NoError(t, err)

	c := &testCache{Cache: cache, tmpDir: tmpDir}
	t.Cleanup(c.cleanup)
	return c
}

func TestCache(t *testing.T) {
	c := newTestCache(t)

	idA := build.ID{'a'}

	path, commit, _, err := c.Create(idA)
	require.NoError(t, err)

	_, _, _, err = c.Create(idA)
	require.Truef(t, errors.Is(err, artifact.ErrWriteLocked), "%v", err)

	_, err = os.Create(filepath.Join(path, "a.txt"))
	require.NoError(t, err)

	require.NoError(t, commit())

	path, unlock, err := c.Get(idA)
	require.NoError(t, err)
	defer unlock()

	_, err = os.Stat(filepath.Join(path, "a.txt"))
	require.NoError(t, err)

	require.Truef(t, errors.Is(c.Remove(idA), artifact.ErrReadLocked), "%v", err)

	idB := build.ID{'b'}
	_, _, err = c.Get(idB)
	require.Truef(t, errors.Is(err, artifact.ErrNotFound), "%v", err)

	require.NoError(t, c.Range(func(artifact build.ID) error {
		require.Equal(t, idA, artifact)
		return nil
	}))
}

func TestAbortWrite(t *testing.T) {
	c := newTestCache(t)

	idA := build.ID{'a'}

	_, _, abort, err := c.Create(idA)
	require.NoError(t, err)
	require.NoError(t, abort())

	_, _, err = c.Get(idA)
	require.Truef(t, errors.Is(err, artifact.ErrNotFound), "%v", err)
}

func TestArtifactExists(t *testing.T) {
	c := newTestCache(t)

	idA := build.ID{'a'}

	_, commit, _, err := c.Create(idA)
	require.NoError(t, err)
	require.NoError(t, commit())

	_, _, _, err = c.Create(idA)
	require.Truef(t, errors.Is(err, artifact.ErrExists), "%v", err)
}
