package hermes

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
)

var errUnauthorized = errors.New("hermes dashboard request unauthorized")

func (c *Client) requestJSON(ctx context.Context, method, path, query string, body any, result any) error {
	if err := c.ensureAuthenticated(ctx); err != nil {
		return err
	}

	err := c.doJSON(ctx, method, path, query, body, result)
	if err == nil || !errors.Is(err, errUnauthorized) {
		return err
	}

	// Basic sessions can expire during a long OpenTofu run. Re-login once,
	// then retry the original request.
	c.authMu.Lock()
	c.authenticated = false
	c.authMu.Unlock()

	if err := c.ensureAuthenticated(ctx); err != nil {
		return err
	}
	return c.doJSON(ctx, method, path, query, body, result)
}

func (c *Client) ensureAuthenticated(ctx context.Context) error {
	c.authMu.Lock()
	if c.authenticated {
		c.authMu.Unlock()
		return nil
	}
	defer c.authMu.Unlock()

	if c.authenticated {
		return nil
	}

	payload := struct {
		Provider string `json:"provider"`
		Username string `json:"username"`
		Password string `json:"password"`
	}{
		Provider: "basic",
		Username: c.username,
		Password: c.password,
	}
	encoded, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("encode Hermes login request: %w", err)
	}

	request, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/auth/password-login", bytes.NewReader(encoded))
	if err != nil {
		return fmt.Errorf("create Hermes login request: %w", err)
	}
	request.Header.Set("Content-Type", "application/json")
	request.Header.Set("Accept", "application/json")
	request.Header.Set("User-Agent", c.userAgent)

	response, err := c.http.Do(request)
	if err != nil {
		return fmt.Errorf("basic Hermes login failed: %w", err)
	}
	defer func() { _ = response.Body.Close() }()

	if response.StatusCode < 200 || response.StatusCode >= 300 {
		return fmt.Errorf("basic Hermes login failed with HTTP %d", response.StatusCode)
	}
	c.authenticated = true
	return nil
}
