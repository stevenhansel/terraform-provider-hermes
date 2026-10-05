package hermes

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
)

// AgentPlugin is the sanitized lifecycle row returned by Hermes' plugins hub.
// Path and dashboard manifest details are intentionally not exposed by the
// provider resource because they are host-local implementation details.
type AgentPlugin struct {
	Name                 string `json:"name"`
	Version              string `json:"version"`
	Description          string `json:"description"`
	Source               string `json:"source"`
	RuntimeStatus        string `json:"runtime_status"`
	HasDashboardManifest bool   `json:"has_dashboard_manifest"`
	CanRemove            bool   `json:"can_remove"`
	CanUpdateGit         bool   `json:"can_update_git"`
	AuthRequired         bool   `json:"auth_required"`
	AuthCommand          string `json:"auth_command"`
	UserHidden           bool   `json:"user_hidden"`
	RemovedReason        string `json:"removed_reason"`
}

// PluginCatalogEntry is a curated catalog row merged with installed state.
type PluginCatalogEntry struct {
	Name            string `json:"name"`
	Repo            string `json:"repo"`
	Subdir          string `json:"subdir"`
	SHA             string `json:"sha"`
	SHAShort        string `json:"sha_short"`
	Installed       bool   `json:"installed"`
	InstalledSHA    string `json:"installed_sha"`
	UpdateAvailable bool   `json:"update_available"`
	RuntimeStatus   string `json:"runtime_status"`
	Tier            string `json:"tier"`
	Description     string `json:"description"`
}

// AgentPluginHub is the unified agent/dashboard plugin response. The
// additional fields are intentionally left out of the public provider state.
type AgentPluginHub struct {
	Plugins []AgentPlugin `json:"plugins"`
}

type pluginCatalogResponse struct {
	Entries []PluginCatalogEntry `json:"entries"`
}

// AgentPluginInstallRequest is the explicit plugin install contract. Hermes
// performs its own catalog/blocklist/security scan; Force is kept visible so
// a caller cannot accidentally bypass that decision through an opaque flag.
type AgentPluginInstallRequest struct {
	Identifier  string `json:"identifier,omitempty"`
	Force       bool   `json:"force"`
	Enable      bool   `json:"enable"`
	CatalogName string `json:"catalog_name,omitempty"`
	Ref         string `json:"ref,omitempty"`
}

// AgentPluginMutationResult is the sanitized result of an agent-plugin
// lifecycle operation. Hermes may return warnings and missing environment
// variable names, but never expose secret values here.
type AgentPluginMutationResult struct {
	OK          bool     `json:"ok"`
	PluginName  string   `json:"plugin_name"`
	Name        string   `json:"name"`
	Warnings    []string `json:"warnings"`
	MissingEnv  []string `json:"missing_env"`
	Enabled     bool     `json:"enabled"`
	Unchanged   bool     `json:"unchanged"`
	Error       string   `json:"error"`
	ScanBlocked bool     `json:"scan_blocked"`
	ScanVerdict string   `json:"scan_verdict"`
}

// ListAgentPlugins returns the lifecycle rows from Hermes' authenticated
// plugin hub. It does not trigger plugin discovery actions.
func (c *Client) ListAgentPlugins(ctx context.Context) ([]AgentPlugin, error) {
	var result AgentPluginHub
	if err := c.requestJSON(ctx, http.MethodGet, "/api/dashboard/plugins/hub", "", nil, &result); err != nil {
		return nil, err
	}
	return result.Plugins, nil
}

// ListPluginCatalog returns curated catalog state. It is read-only and is
// useful for matching catalog identifiers whose installed manifest name
// differs from the catalog name.
func (c *Client) ListPluginCatalog(ctx context.Context) ([]PluginCatalogEntry, error) {
	var result pluginCatalogResponse
	if err := c.requestJSON(ctx, http.MethodGet, "/api/dashboard/plugins/catalog", "", nil, &result); err != nil {
		return nil, err
	}
	return result.Entries, nil
}

// InstallAgentPlugin performs Hermes' synchronous, scanned install operation.
func (c *Client) InstallAgentPlugin(ctx context.Context, request AgentPluginInstallRequest) (AgentPluginMutationResult, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	var result AgentPluginMutationResult
	if err := c.requestJSON(ctx, http.MethodPost, "/api/dashboard/agent-plugins/install", "", request, &result); err != nil {
		return AgentPluginMutationResult{}, err
	}
	if !result.OK {
		return result, pluginMutationError("installation", result)
	}
	return result, nil
}

// SetAgentPluginEnabled changes only the runtime allow/deny state.
func (c *Client) SetAgentPluginEnabled(ctx context.Context, name string, enabled bool) (AgentPluginMutationResult, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	action := "disable"
	if enabled {
		action = "enable"
	}
	return c.pluginMutation(ctx, http.MethodPost, "/api/dashboard/agent-plugins/"+url.PathEscape(name)+"/"+action)
}

// UpdateAgentPlugin updates a user-installed plugin from its source or
// catalog pin. Hermes handles the source-specific safety checks.
func (c *Client) UpdateAgentPlugin(ctx context.Context, name string) (AgentPluginMutationResult, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return c.pluginMutation(ctx, http.MethodPost, "/api/dashboard/agent-plugins/"+url.PathEscape(name)+"/update")
}

// DeleteAgentPlugin removes only a user-installed plugin tree. Bundled
// plugins are rejected by Hermes and therefore cannot be accidentally owned.
func (c *Client) DeleteAgentPlugin(ctx context.Context, name string) (AgentPluginMutationResult, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return c.pluginMutation(ctx, http.MethodDelete, "/api/dashboard/agent-plugins/"+url.PathEscape(name))
}

func (c *Client) pluginMutation(ctx context.Context, method, path string) (AgentPluginMutationResult, error) {
	var result AgentPluginMutationResult
	if err := c.requestJSON(ctx, method, path, "", nil, &result); err != nil {
		return AgentPluginMutationResult{}, err
	}
	if !result.OK {
		return result, pluginMutationError("mutation", result)
	}
	return result, nil
}

func pluginMutationError(operation string, result AgentPluginMutationResult) error {
	message := strings.TrimSpace(result.Error)
	if message == "" {
		message = "Hermes rejected the plugin operation"
	}
	if result.ScanBlocked && strings.TrimSpace(result.ScanVerdict) != "" {
		message = fmt.Sprintf("%s (scan verdict: %s)", message, result.ScanVerdict)
	}
	return fmt.Errorf("hermes plugin %s failed: %s", operation, message)
}
