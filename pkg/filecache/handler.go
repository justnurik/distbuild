//go:build !solution

package filecache

import (
	"fmt"
	"io"
	"net/http"
	"os"
	"sync"

	"gitlab.com/justnurik/distbuild/pkg/build"
	"gitlab.com/justnurik/distbuild/pkg/concurrency"
	"go.uber.org/zap"
	"golang.org/x/sync/singleflight"
)

type Handler struct {
	l      *zap.Logger
	cache  *Cache
	flight singleflight.Group
	lock   *concurrency.KeyLocker[build.ID]
}

func NewHandler(l *zap.Logger, cache *Cache) *Handler {
	return &Handler{
		l:     l.With(zap.String("component", "filecache_handler")),
		cache: cache,
		lock:  concurrency.NewKeyLocker[build.ID](&sync.Mutex{}),
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/file", func(w http.ResponseWriter, r *http.Request) {
		strID := r.URL.Query().Get("id")
		if strID == "" {
			h.l.Error("file id required")
			http.Error(w, "file id required", http.StatusBadRequest)
			return
		}

		var fileID build.ID
		if err := fileID.UnmarshalText([]byte(strID)); err != nil {
			h.l.Error("invalid file id",
				zap.String("id", strID),
				zap.Error(err))
			http.Error(w, "invalid file id", http.StatusBadRequest)
			return
		}

		switch r.Method {

		case http.MethodGet:
			h.handleGet(fileID, w)

		case http.MethodPut:
			h.handlePut(fileID, w, r)

		default:
			h.l.Error("unsupported method", zap.String("method", r.Method))
			http.Error(w, "GET or PUT request", http.StatusMethodNotAllowed)

		}
	})
}

func (h *Handler) handleGet(fileID build.ID, w http.ResponseWriter) {
	_, err, _ := h.flight.Do(fileID.String(), func() (any, error) {
		path, unlock, err := h.cache.Get(fileID)
		if err != nil {
			h.l.Error("failed to get file from cache",
				zap.String("id", fileID.String()),
				zap.Error(err))
			return nil, fmt.Errorf("failed to get file from cache: %w", err)
		}
		defer unlock()

		file, err := os.Open(path)
		if err != nil {
			h.l.Error("couldn't open the file", zap.Error(err))
			return nil, fmt.Errorf("couldn't open the file: %w", err)
		}
		defer func() { _ = file.Close() }()

		w.Header().Set("Content-Type", "application/octet-stream")

		if n, err := io.Copy(w, file); err != nil {
			h.l.Error("couldn't copy the file to the request body",
				zap.String("id", fileID.String()),
				zap.Error(err),
				zap.Int64("bytes copied", n))
			return nil, fmt.Errorf("couldn't copy the file to the request body: %w", err)
		}

		return nil, nil
	})
	if err != nil {
		h.l.Error("the error occurred inside `h.flight.Do`", zap.Error(err))
		http.Error(w, err.Error(), http.StatusInternalServerError)
	}
}

func (h *Handler) handlePut(fileID build.ID, w http.ResponseWriter, r *http.Request) {
	h.lock.Lock(fileID)
	defer h.lock.Unlock(fileID)

start:
	writer, abort, err := h.cache.Write(fileID)

	switch err {

	case ErrExists:

		if err := h.cache.Remove(fileID); err != nil {
			h.l.Error("couldn't delete the file to update the data",
				zap.String("id", fileID.String()),
				zap.Error(err))
			http.Error(w, fmt.Errorf("couldn't delete the file to update the data: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		goto start //! one-shot

	case nil:

		defer func() {
			if err != nil {
				_ = abort()
			}
		}()

	default:

		h.l.Error("failed to create a cache entry",
			zap.String("id", fileID.String()),
			zap.Error(err))
		http.Error(w, fmt.Errorf("failed to create a cache entry: %w", err).Error(), http.StatusInternalServerError)
		return
	}

	if _, err = io.Copy(writer, r.Body); err != nil {
		h.l.Error("couldn't copy data to cache", zap.Error(err))
		http.Error(w, fmt.Errorf("couldn't copy data to cache: %w", err).Error(), http.StatusInternalServerError)
		return
	}

	if err = writer.Close(); err != nil {
		h.l.Error("failed to close writer", zap.Error(err))
		http.Error(w, fmt.Errorf("close writer: %w", err).Error(), http.StatusInternalServerError)
	}

	h.l.Info("file successfully uploaded", zap.String("id", fileID.String()))
}
