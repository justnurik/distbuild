package api

import (
	"encoding/json"
	"fmt"
	"io"

	"go.uber.org/zap"
)

type streamStatusReader struct {
	decoder  *json.Decoder
	respBody io.ReadCloser
	l        *zap.Logger
}

func NewStatusReader(decoder *json.Decoder, respBody io.ReadCloser, l *zap.Logger) StatusReader {
	return &streamStatusReader{
		decoder:  decoder,
		respBody: respBody,
		l:        l.With(zap.String("component", "status_reader")),
	}
}

func (r *streamStatusReader) Next() (*StatusUpdate, error) {
	var update StatusUpdate

	if err := r.decoder.Decode(&update); err != nil {
		if err == io.EOF {
			r.l.Info("streaming connection closed", zap.String("event", "eof"))
			return nil, io.EOF
		}

		r.l.Error("decode message failed", zap.Error(err))
		return nil, fmt.Errorf("decode message failed: %w", err)
	}

	return &update, nil
}

func (r *streamStatusReader) Close() error {
	r.l.Info("status reader closed",
		zap.String("component", "stream_reader"),
		zap.String("action", "cleanup"))
	return r.respBody.Close()
}
