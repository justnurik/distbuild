//go:build !solution

package api

import (
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

type HeartbeatHandler struct {
	l *zap.Logger
	s HeartbeatService
}

func NewHeartbeatHandler(l *zap.Logger, s HeartbeatService) *HeartbeatHandler {
	return &HeartbeatHandler{
		l: l.With(zap.String("component", "heartbeat_handler")),
		s: s,
	}
}

func (h *HeartbeatHandler) Register(mux *http.ServeMux) {
	mux.HandleFunc("/heartbeat", func(w http.ResponseWriter, r *http.Request) {
		var req HeartbeatRequest
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			h.l.Error("request body decoding error", zap.Error(err))
			http.Error(w, fmt.Sprintf("request body decoding error: %v", err), http.StatusBadRequest)
			return
		}

		resp, err := h.s.Heartbeat(r.Context(), &req)
		if err != nil {
			h.l.Error("error on the coordinator's side: build execution error",
				zap.Error(err),
				zap.Any("HeartbeatResponse", resp))
			http.Error(w, fmt.Errorf("error before streaming started: %w", err).Error(), http.StatusInternalServerError)
			return
		}

		if err := json.NewEncoder(w).Encode(*resp); err != nil {
			h.l.Error("request body decoding error", zap.Error(err))
			http.Error(w, fmt.Sprintf("request body decoding error: %v", err), http.StatusBadRequest)
		}
	})
}
