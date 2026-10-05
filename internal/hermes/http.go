package hermes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

func (c *Client) doJSON(ctx context.Context, method, path, query string, body any, result any) error {
	var payload io.Reader
	if body != nil {
		encoded, err := json.Marshal(body)
		if err != nil {
			return fmt.Errorf("encode Hermes request: %w", err)
		}
		payload = bytes.NewReader(encoded)
	}

	endpoint := c.baseURL + path
	if query != "" {
		endpoint += "?" + query
	}
	request, err := http.NewRequestWithContext(ctx, method, endpoint, payload)
	if err != nil {
		return fmt.Errorf("create Hermes request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", c.userAgent)
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}

	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("hermes request %s %s failed: %w", method, path, err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode == http.StatusUnauthorized || response.StatusCode == http.StatusForbidden {
		return errUnauthorized
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		// Do not include the response body: Hermes error pages can contain
		// configuration values or authentication details.
		return &HTTPError{Method: method, Path: path, StatusCode: response.StatusCode}
	}
	if result == nil {
		return nil
	}

	limited := io.LimitReader(response.Body, maxResponseBytes)
	if err := json.NewDecoder(limited).Decode(result); err != nil {
		return fmt.Errorf("decode Hermes response for %s %s: %w", method, path, err)
	}
	return nil
}
