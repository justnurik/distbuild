//go:build !solution

package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"gitlab.com/justnurik/distbuild/pkg/build"
	"go.uber.org/zap"
)

type BuildHandler struct {
	l *zap.Logger
	s Service
}

func NewBuildService(l *zap.Logger, s Service) *BuildHandler {
	return &BuildHandler{
		l: l.With(zap.String("component", "build_handler")),
		s: s,
	}
}

func (h *BuildHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/build", h.startBuildHandler)
	mux.HandleFunc("/signal", h.signalBuildHandler)
}

func (h *BuildHandler) startBuildHandler(w http.ResponseWriter, r *http.Request) {

	buildRequest := &BuildRequest{}
	if err := json.NewDecoder(r.Body).Decode(&buildRequest.Graph); err != nil {
		h.l.Error("request body decoding error",
			zap.Error(err))
		http.Error(w, fmt.Sprintf("request body decoding error: %v", err), http.StatusBadRequest)
		return
	}

	sw := NewStatusWriter(h.l, w)

	err := h.s.StartBuild(r.Context(), buildRequest, sw)
	if err != nil {
		h.l.Error("error on the coordinator's side: build execution error",
			zap.Error(err),
			zap.Any("graph", buildRequest.Graph))

		if sw.startMsgSend.Load() {
			update := &StatusUpdate{
				BuildFailed: &BuildFailed{
					Error: err.Error(),
				},
			}

			if sendErr := sw.Updated(update); sendErr != nil {
				h.l.Error("couldn't send error via streaming",
					zap.Error(sendErr),
					zap.NamedError("original_error", err))
			}
			h.l.Error("error after streaming started", zap.Error(err))
			return
		}

		h.l.Error("error before streaming started", zap.Error(err))
		http.Error(w, fmt.Errorf("error before streaming started: %w", err).Error(), http.StatusInternalServerError)
		return
	}

	sw.mu.Lock()
	done := sw.done
	sw.mu.Unlock()

	<-done
	h.l.Info("successful start of the build", zap.Any("graph", buildRequest.Graph))
}

func (h *BuildHandler) signalBuildHandler(w http.ResponseWriter, r *http.Request) {

	strID := r.URL.Query().Get("build_id")
	if strID == "" {
		h.l.Error("build_id не указан")
		http.Error(w, "build id required", http.StatusBadRequest)
		return
	}

	var buildID build.ID
	if err := buildID.UnmarshalText([]byte(strID)); err != nil {
		h.l.Error("invalid build id",
			zap.String("id", strID),
			zap.Error(err),
		)
		http.Error(w, "invalid build id", http.StatusBadRequest)
		return
	}

	signalRequest := &SignalRequest{}
	if err := json.NewDecoder(r.Body).Decode(&signalRequest); err != nil {
		h.l.Error("request body decoding error",
			zap.Error(err),
			zap.String("build_id", buildID.String()))
		http.Error(w, fmt.Sprintf("request body decoding error: %v", err), http.StatusBadRequest)
		return
	}

	_, err := h.s.SignalBuild(r.Context(), buildID, signalRequest)
	if err != nil {
		h.l.Error("error on the coordinator's side: signal execution error",
			zap.Error(err),
			zap.String("build_id", buildID.String()))
		http.Error(w, fmt.Errorf("error on the coordinator's side: signal execution error: %w", err).Error(), http.StatusInternalServerError)
		return
	}
}
