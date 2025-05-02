package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"sync/atomic"

	"go.uber.org/zap"
)

type streamStatusWriter struct {
	mu sync.Mutex

	rc *http.ResponseController
	w  http.ResponseWriter

	done chan struct{}

	// for looger
	l            *zap.Logger
	startMsgSend atomic.Bool
}

func NewStatusWriter(l *zap.Logger, w http.ResponseWriter) *streamStatusWriter {
	rc := http.NewResponseController(w)
	w.Header().Set("Content-Type", "application/json")

	return &streamStatusWriter{
		w:  w,
		rc: rc,

		done: make(chan struct{}),

		l:            l.With(zap.String("component", "status_writer")),
		startMsgSend: atomic.Bool{},
	}
}

func (s *streamStatusWriter) send(msg any) error {
	err := json.NewEncoder(s.w).Encode(msg)
	if err != nil {
		s.l.Error("failed to write message",
			zap.Error(err))
		return fmt.Errorf("write failed: %w", err)
	}

	if err := s.rc.Flush(); err != nil {
		s.l.Error("flush failed",
			zap.Error(err))
		return fmt.Errorf("flush failed: %w", err)
	}

	return nil
}

func (s *streamStatusWriter) Started(rsp *BuildStarted) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.startMsgSend.CompareAndSwap(false, true) {
		s.l.Error("attempt to send started message twice")
		return fmt.Errorf("started message already sent")
	}

	if err := s.send(*rsp); err != nil {
		s.l.Error("failed to send started message",
			zap.Error(err))
		return fmt.Errorf("send started failed: %w", err)
	}

	s.l.Info("build started message sent",
		zap.String("build_id", rsp.ID.String()))

	return nil
}

func (s *streamStatusWriter) Updated(update *StatusUpdate) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.startMsgSend.Load() {
		s.l.Error("attempt to send update before started")
		return fmt.Errorf("started message not sent")
	}

	if err := s.send(update); err != nil {
		s.l.Error("failed to send status update",
			zap.Error(err),
			zap.Any("update", update))

		return fmt.Errorf("send update failed: %w", err)
	}

	if update.BuildFinished != nil {
		close(s.done)
		s.l.Info("build finished",
			zap.Bool("success", update.JobFinished == nil),
			zap.Any("error", update.BuildFailed))
	}

	return nil
}
