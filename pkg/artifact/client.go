//go:build !solution

package artifact

import (
	"context"
	"fmt"
	"io"
	"net/http"

	"gitlab.com/justnurik/distbuild/pkg/build"
	"gitlab.com/justnurik/distbuild/pkg/tarstream"
)

// Download artifact from remote cache into local cache.
func Download(ctx context.Context, endpoint string, c *Cache, artifactID build.ID) error {
	url := fmt.Sprintf("%s/artifact?id=%s", endpoint, artifactID.String())

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return fmt.Errorf("error creating GET %s request: %w", url, err)
	}

	client := http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("error sending the request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("unexpected status: %d, body: %s", resp.StatusCode, string(body))
	}

	path, commit, abort, err := c.Create(artifactID)
	if err != nil {
		return fmt.Errorf("create cache entry: %w", err)
	}

	if err := tarstream.Receive(path, resp.Body); err != nil {
		abortErr := abort()
		return fmt.Errorf("receive tar stream: %w, abort err: %w", err, abortErr)
	}

	if err := commit(); err != nil {
		return fmt.Errorf("commit artifact: %w", err)
	}

	return nil
}
