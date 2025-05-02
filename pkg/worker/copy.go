package worker

import (
	"os"
	"path/filepath"

	"gitlab.com/justnurik/distbuild/pkg/build"
	"gitlab.com/justnurik/distbuild/pkg/filecache"
)

func linkFiles(from *filecache.Cache, toDir string, fileID build.ID, filePath string) error {
	path, unlock, err := from.Get(fileID)
	if err != nil {
		return err
	}
	defer unlock()

	newFilePath := filepath.Join(toDir, filePath)
	if err := os.MkdirAll(filepath.Dir(newFilePath), 0755); err != nil {
		return err
	}
	_ = os.Remove(newFilePath)

	if err := os.Link(path, newFilePath); err != nil {
		return err
	}

	return nil
}
