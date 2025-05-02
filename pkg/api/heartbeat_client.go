//go:build !solution

package api

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"go.uber.org/zap"
)

type HeartbeatClient struct {
	endpoint string
	client   http.Client
	l        *zap.Logger
}

func NewHeartbeatClient(l *zap.Logger, endpoint string) *HeartbeatClient {
	return &HeartbeatClient{
		client: http.Client{
			Transport: &http.Transport{
				MaxIdleConns:    100,
				IdleConnTimeout: 90 * time.Second,
			},
		},
		endpoint: endpoint,
		l:        l.With(zap.String("component", "heartbeat_client")),
	}
}

func (c *HeartbeatClient) Heartbeat(ctx context.Context, req *HeartbeatRequest) (*HeartbeatResponse, error) {
	responce, err := doRequest(c.l, &c.client, ctx, *req, c.endpoint+"/heartbeat")
	if err != nil {
		return nil, err
	}
	defer func() { _ = responce.Body.Close() }()

	if responce.StatusCode != http.StatusOK {
		body, err := io.ReadAll(responce.Body)
		if err != nil {
			c.l.Error("couldn't read the error text", zap.Error(err))
			return nil, fmt.Errorf("read error body: %w", err)
		}

		c.l.Error("unexpected status",
			zap.Int("status", responce.StatusCode),
			zap.ByteString("body", body))
		return nil, fmt.Errorf("unexpected status: %d, Body: %s", responce.StatusCode, string(body))
	}

	var resp HeartbeatResponse
	if err := json.NewDecoder(responce.Body).Decode(&resp); err != nil {
		c.l.Error("decode first message failed", zap.Error(err))
		return nil, fmt.Errorf("decode first message failed: %w", err)
	}
	c.l.Info("", zap.String("endpoint", c.endpoint+"/heartbeat"),
		zap.Any("responce", resp))
	return &resp, nil
}
