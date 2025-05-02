package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"

	"go.uber.org/zap"
)

func doRequest(l *zap.Logger, client *http.Client, ctx context.Context, requestData any, url string) (*http.Response, error) {
	data, err := json.Marshal(requestData)
	if err != nil {
		l.Error("request body encoding error", zap.Error(err))
		return nil, fmt.Errorf("request body encoding error: %w", err)
	}
	req, err := http.NewRequestWithContext(
		ctx, "POST", url, bytes.NewReader(data))
	if err != nil {
		l.Error(fmt.Sprintf("error creating a POST %s request", url),
			zap.Error(err))
		return nil, fmt.Errorf("error creating a POST %s request: %w", url, err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := client.Do(req)
	if err != nil {
		l.Error("error sending the request", zap.Error(err))
		return nil, fmt.Errorf("error sending the request: %w", err)
	}

	return resp, nil
}
