package hermes

import (
	"context"
	"net/http"
	"net/url"
)

// MessagingEnvVar describes one non-secret environment variable in Hermes'
// fixed messaging-platform catalog. Hermes deliberately returns only whether
// a value is set; the actual value is never decoded by the provider.
type MessagingEnvVar struct {
	Key        string `json:"key"`
	Required   bool   `json:"required"`
	IsSet      bool   `json:"is_set"`
	IsPassword bool   `json:"is_password"`
}

// MessagingPlatform is the non-secret platform status returned by Hermes.
type MessagingPlatform struct {
	ID             string            `json:"id"`
	Name           string            `json:"name"`
	Description    string            `json:"description"`
	DocsURL        string            `json:"docs_url"`
	Enabled        bool              `json:"enabled"`
	Configured     bool              `json:"configured"`
	GatewayRunning bool              `json:"gateway_running"`
	State          string            `json:"state"`
	ErrorCode      string            `json:"error_code"`
	UpdatedAt      *string           `json:"updated_at"`
	EnvVars        []MessagingEnvVar `json:"env_vars"`
}

type messagingPlatformsResponse struct {
	Platforms []MessagingPlatform `json:"platforms"`
}

// MessagingPlatformUpdateRequest contains only the endpoint-specific fields
// needed by the resource. Env values are resolved immediately before this
// request and are never represented in Terraform/OpenTofu state.
type MessagingPlatformUpdateRequest struct {
	Enabled  *bool             `json:"enabled,omitempty"`
	Env      map[string]string `json:"env,omitempty"`
	ClearEnv []string          `json:"clear_env,omitempty"`
}

// ListMessagingPlatforms reads Hermes' fixed messaging platform catalog.
func (c *Client) ListMessagingPlatforms(ctx context.Context, profile string) ([]MessagingPlatform, error) {
	var result messagingPlatformsResponse
	if err := c.requestJSON(ctx, http.MethodGet, "/api/messaging/platforms", profileQuery(profile), nil, &result); err != nil {
		return nil, err
	}
	return result.Platforms, nil
}

// UpdateMessagingPlatform updates one fixed platform. The Hermes route owns
// env-file/config persistence and validates platform-specific keys.
func (c *Client) UpdateMessagingPlatform(ctx context.Context, profile, platform string, update MessagingPlatformUpdateRequest) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return c.requestJSON(ctx, http.MethodPut, "/api/messaging/platforms/"+url.PathEscape(platform), profileQuery(profile), update, nil)
}

// TestMessagingPlatform performs Hermes' explicit connectivity/status check.
// Resources intentionally do not call it during Read or refresh; it remains a
// separate client operation for a future explicit action surface.
func (c *Client) TestMessagingPlatform(ctx context.Context, profile, platform string) (MessagingPlatformTestResult, error) {
	var result MessagingPlatformTestResult
	if err := c.requestJSON(ctx, http.MethodPost, "/api/messaging/platforms/"+url.PathEscape(platform)+"/test", profileQuery(profile), nil, &result); err != nil {
		return MessagingPlatformTestResult{}, err
	}
	return result, nil
}

// MessagingPlatformTestResult is returned by Hermes' explicit platform test.
type MessagingPlatformTestResult struct {
	OK      bool   `json:"ok"`
	State   string `json:"state"`
	Message string `json:"message"`
}
