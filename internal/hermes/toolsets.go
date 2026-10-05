package hermes

import (
	"context"
	"net/http"
	"net/url"
)

// Toolset is a built-in, configurable Hermes capability group. Toolsets are
// toggles rather than independently created remote objects.
type Toolset struct {
	Name          string   `json:"name"`
	Label         string   `json:"label"`
	Description   string   `json:"description"`
	Platform      string   `json:"platform"`
	PlatformLabel string   `json:"platform_label"`
	Enabled       bool     `json:"enabled"`
	Available     bool     `json:"available"`
	Configured    bool     `json:"configured"`
	Tools         []string `json:"tools"`
}

// ToolsetToggleResult is the response from Hermes after changing a toolset.
// Enabling some toolsets may start an asynchronous post-setup operation.
type ToolsetToggleResult struct {
	OK               bool    `json:"ok"`
	Name             string  `json:"name"`
	Platform         string  `json:"platform"`
	Enabled          bool    `json:"enabled"`
	PostSetupStarted *string `json:"post_setup_started"`
}

type toolsetToggleRequest struct {
	Enabled bool   `json:"enabled"`
	Profile string `json:"profile,omitempty"`
}

// ListToolsets returns Hermes' built-in configurable toolsets for a profile.
func (c *Client) ListToolsets(ctx context.Context, profile string) ([]Toolset, error) {
	var result []Toolset
	if err := c.requestJSON(ctx, http.MethodGet, "/api/tools/toolsets", profileQuery(profile), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetToolsetEnabled changes only a built-in toolset toggle. Hermes may start
// a post-setup action as a side effect when enabling a capability; that action
// is surfaced to state but is not silently polled or retried here.
func (c *Client) SetToolsetEnabled(ctx context.Context, profile, name string, enabled bool) (ToolsetToggleResult, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	var result ToolsetToggleResult
	body := toolsetToggleRequest{Enabled: enabled, Profile: profile}
	path := "/api/tools/toolsets/" + url.PathEscape(name)
	if err := c.requestJSON(ctx, http.MethodPut, path, profileQuery(profile), body, &result); err != nil {
		return ToolsetToggleResult{}, err
	}
	return result, nil
}
