//go:build !solution

package artifact

import (
	"fmt"
	"net/http"

	"gitlab.com/justnurik/distbuild/pkg/build"
	"gitlab.com/justnurik/distbuild/pkg/tarstream"
	"go.uber.org/zap"
)

type Handler struct {
	l *zap.Logger
	c *Cache
}

func NewHandler(l *zap.Logger, c *Cache) *Handler {
	return &Handler{
		l: l.With(zap.String("component", "artifact_handler")),
		c: c,
	}
}

func (h *Handler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/artifact", func(w http.ResponseWriter, r *http.Request) {
		strID := r.URL.Query().Get("id")
		if strID == "" {
			h.l.Error("artifact id required")
			http.Error(w, "artifact id required", http.StatusBadRequest)
			return
		}

		var artifactID build.ID
		if err := artifactID.UnmarshalText([]byte(strID)); err != nil {
			h.l.Error("invalid artifact id",
				zap.String("id", strID),
				zap.Error(err))
			http.Error(w, "invalid artifact id", http.StatusBadRequest)
			return
		}

		path, unlock, err := h.c.Get(artifactID)
		if err != nil {
			if err == ErrNotFound {
				h.l.Error("artifact not found", zap.String("id", strID))
				http.Error(w, "artifact not found", http.StatusNotFound)
				return
			}

			h.l.Error("failed to get artifact",
				zap.String("id", strID),
				zap.Error(err))
			http.Error(w, fmt.Errorf("internal server error: %w", err).Error(), http.StatusInternalServerError)
			return
		}
		defer unlock()

		if err := tarstream.Send(path, w); err != nil {
			h.l.Error("failed to send artifact",
				zap.String("id", strID),
				zap.Error(err))
			http.Error(w, fmt.Errorf("failed to send artifact: %w", err).Error(), http.StatusBadRequest)
		}
	})
}
