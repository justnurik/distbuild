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

	"gitlab.com/justnurik/distbuild/pkg/build"
)

type BuildClient struct {
	endpoint string
	client   http.Client
	l        *zap.Logger
}

func NewBuildClient(l *zap.Logger, endpoint string) *BuildClient {
	return &BuildClient{
		client: http.Client{
			Transport: &http.Transport{
				MaxIdleConns:    1000,
				IdleConnTimeout: 90 * time.Second,
			},
		},
		endpoint: endpoint,
		l:        l.With(zap.String("component", "build_client")),
	}
}

func (c *BuildClient) StartBuild(ctx context.Context, request *BuildRequest) (*BuildStarted, StatusReader, error) {

	resp, err := doRequest(c.l, &c.client, ctx, request.Graph, c.endpoint+"/build")
	if err != nil {
		return nil, nil, err
	}

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		_ = resp.Body.Close()

		if err != nil {
			c.l.Error("read error body",
				zap.Error(err))
			return nil, nil, fmt.Errorf("read error body: %w", err)
		}

		c.l.Error("unexpected status",
			zap.Int("status", resp.StatusCode),
			zap.ByteString("body", body))
		return nil, nil, fmt.Errorf("unexpected status: %d, Body: %s", resp.StatusCode, string(body))
	}

	decoder := json.NewDecoder(resp.Body)

	var started BuildStarted
	if err := decoder.Decode(&started); err != nil {
		_ = resp.Body.Close()

		c.l.Error("decode first message failed",
			zap.Error(err))
		return nil, nil, fmt.Errorf("decode first message failed: %w", err)
	}

	return &started, NewStatusReader(decoder, resp.Body, c.l), nil
}

func (c *BuildClient) SignalBuild(ctx context.Context, buildID build.ID, signal *SignalRequest) (*SignalResponse, error) {

	resp, err := doRequest(c.l, &c.client, ctx, signal, fmt.Sprintf("%s/signal?build_id=%s", c.endpoint, buildID.String()))
	if err != nil {
		return nil, err
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode == http.StatusOK {
		return &SignalResponse{}, nil
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.l.Error("read error body",
			zap.Error(err))
		return nil, fmt.Errorf("read error body: %w", err)
	}

	c.l.Error("unexpected status",
		zap.Int("status", resp.StatusCode),
		zap.ByteString("body", body))
	return nil, fmt.Errorf("unexpected status: %d, Body: %s", resp.StatusCode, string(body))
}
