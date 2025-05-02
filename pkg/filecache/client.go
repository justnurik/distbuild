//go:build !solution

package filecache

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"

	"go.uber.org/zap"

	"gitlab.com/justnurik/distbuild/pkg/build"
)

type Client struct {
	endpoint string
	client   http.Client
	l        *zap.Logger
}

func NewClient(l *zap.Logger, endpoint string) *Client {
	return &Client{
		endpoint: endpoint,
		client:   http.Client{Transport: &http.Transport{MaxConnsPerHost: 200}},
		l:        l.With(zap.String("component", "filecache_client")),
	}
}

func (c *Client) Upload(ctx context.Context, fileID build.ID, localPath string) error {
	file, err := os.Open(localPath)
	if err != nil {
		c.l.Error("couldn't open the file", zap.Error(err))
		return fmt.Errorf("couldn't open the file: %w", err)
	}
	defer file.Close()

	url := fmt.Sprintf("%s/file?id=%s", c.endpoint, fileID.String())
	req, err := http.NewRequestWithContext(ctx, "PUT", url, file)
	if err != nil {
		c.l.Error(fmt.Sprintf("failed to creating a PUT %s request", url), zap.Error(err))
		return fmt.Errorf("error creating a PUT %s request: %w", url, err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.client.Do(req)
	if err != nil {
		c.l.Error("failed to sending the request", zap.Error(err))
		return fmt.Errorf("error sending the request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			c.l.Error("read error body", zap.Error(err))
			return fmt.Errorf("read error body: %w", err)
		}

		c.l.Error("unexpected status", zap.Int("status", resp.StatusCode), zap.String("body", string(body)))
		return fmt.Errorf("unexpected status: %d, Body: %s", resp.StatusCode, string(body))
	}

	return nil
}

func (c *Client) Download(ctx context.Context, localCache *Cache, fileID build.ID) error {
	url := fmt.Sprintf("%s/file?id=%s", c.endpoint, fileID.String())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		c.l.Error(fmt.Sprintf("failed to creating a GET %s request", url), zap.Error(err))
		return fmt.Errorf("error creating a GET %s request: %w", url, err)
	}

	req.Header.Set("Content-Type", "application/octet-stream")

	resp, err := c.client.Do(req)
	if err != nil {
		c.l.Error("failed to sending the request", zap.Error(err))
		return fmt.Errorf("error sending the request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		c.l.Error("unexpected status", zap.Int("status", resp.StatusCode))
		return fmt.Errorf("unexpected status: %s", resp.Status)
	}

	w, abort, err := localCache.Write(fileID)
	if err != nil {
		c.l.Error("failed to create cache file", zap.Error(err))
		return fmt.Errorf("create cache file: %w", err)
	}
	defer func() {
		if err != nil {
			_ = abort()
		}
	}()

	if _, err = io.Copy(w, resp.Body); err != nil {
		c.l.Error("couldn't copy data to local cache", zap.Error(err))
		return fmt.Errorf("couldn't copy data to local cache: %w", err)
	}

	if err = w.Close(); err != nil {
		c.l.Error("failed to close writer", zap.Error(err))
		return fmt.Errorf("close writer: %w", err)
	}

	return nil
}
