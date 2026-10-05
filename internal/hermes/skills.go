package hermes

import (
	"context"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Skill is the non-secret metadata returned by Hermes' skill inventory.
type Skill struct {
	Name        string `json:"name"`
	Description string `json:"description"`
	Category    string `json:"category"`
	Enabled     bool   `json:"enabled"`
	Usage       int64  `json:"usage"`
	Provenance  string `json:"provenance"`
}

// SkillHubEntry is a catalog or featured skill row. The provider only uses
// the installation metadata returned by the sources endpoint, but keeping the
// common row typed makes the client useful for future read-only discovery.
type SkillHubEntry struct {
	Identifier  string `json:"identifier"`
	Name        string `json:"name"`
	Description string `json:"description"`
	TrustLevel  string `json:"trust_level"`
	ScanVerdict string `json:"scan_verdict"`
	Installed   bool   `json:"installed"`
}

// SkillHubSource describes one configured skill-hub source.
type SkillHubSource struct {
	ID          string `json:"id"`
	Label       string `json:"label"`
	RateLimited bool   `json:"rate_limited"`
	Available   bool   `json:"available"`
	Searchable  bool   `json:"searchable"`
}

// SkillHubInstallation is the sanitized lock metadata for an installed hub
// skill. It intentionally excludes the skill contents and filesystem path.
type SkillHubInstallation struct {
	Identifier  string `json:"identifier"`
	Name        string `json:"name"`
	TrustLevel  string `json:"trust_level"`
	ScanVerdict string `json:"scan_verdict"`
}

// SkillHubSources is returned by Hermes' skill-hub sources endpoint.
type SkillHubSources struct {
	Sources        []SkillHubSource                `json:"sources"`
	IndexAvailable bool                            `json:"index_available"`
	Featured       []SkillHubEntry                 `json:"featured"`
	Installed      map[string]SkillHubInstallation `json:"installed"`
}

// ActionStatus is the bounded, non-log portion of a Hermes background action
// status response. Lines are deliberately not retained because action logs
// can contain user prompts, credentials, or tool output.
type ActionStatus struct {
	Name     string `json:"name"`
	Running  bool   `json:"running"`
	ExitCode *int   `json:"exit_code"`
	PID      *int   `json:"pid"`
}

type skillHubActionResponse struct {
	OK    bool   `json:"ok"`
	PID   int    `json:"pid"`
	Name  string `json:"name"`
	Error string `json:"error"`
}

type skillToggleRequest struct {
	Name    string `json:"name"`
	Enabled bool   `json:"enabled"`
	Profile string `json:"profile,omitempty"`
}

type skillInstallRequest struct {
	Identifier string `json:"identifier"`
	Profile    string `json:"profile,omitempty"`
}

type skillUninstallRequest struct {
	Name    string `json:"name"`
	Profile string `json:"profile,omitempty"`
}

// ListSkills returns the skills visible to the selected profile. Hermes omits
// disabled skills from this list in current releases; callers must account
// for that when managing a disabled desired state.
func (c *Client) ListSkills(ctx context.Context, profile string) ([]Skill, error) {
	var result []Skill
	if err := c.requestJSON(ctx, http.MethodGet, "/api/skills", profileQuery(profile), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// SetSkillEnabled toggles an existing skill. Hermes persists the disabled
// name set and does not remove the skill itself.
func (c *Client) SetSkillEnabled(ctx context.Context, profile, name string, enabled bool) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	body := skillToggleRequest{Name: name, Enabled: enabled, Profile: profile}
	return c.requestJSON(ctx, http.MethodPut, "/api/skills/toggle", profileQuery(profile), body, nil)
}

// ListSkillHubSources returns hub provenance and installed lock metadata.
func (c *Client) ListSkillHubSources(ctx context.Context, profile string) (SkillHubSources, error) {
	var result SkillHubSources
	if err := c.requestJSON(ctx, http.MethodGet, "/api/skills/hub/sources", profileQuery(profile), nil, &result); err != nil {
		return SkillHubSources{}, err
	}
	if result.Installed == nil {
		result.Installed = map[string]SkillHubInstallation{}
	}
	return result, nil
}

// StartSkillInstall starts Hermes' background, security-scanned skill-hub
// installation and returns its action name for bounded polling.
func (c *Client) StartSkillInstall(ctx context.Context, profile, identifier string) (string, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	var result skillHubActionResponse
	body := skillInstallRequest{Identifier: identifier, Profile: profile}
	if err := c.requestJSON(ctx, http.MethodPost, "/api/skills/hub/install", profileQuery(profile), body, &result); err != nil {
		return "", err
	}
	if !result.OK {
		if strings.TrimSpace(result.Error) == "" {
			return "", fmt.Errorf("hermes rejected skill installation")
		}
		return "", fmt.Errorf("hermes rejected skill installation: %s", result.Error)
	}
	if strings.TrimSpace(result.Name) == "" {
		return "", fmt.Errorf("hermes skill installation response did not include an action name")
	}
	return result.Name, nil
}

// StartSkillUninstall starts Hermes' background skill-hub uninstall action.
func (c *Client) StartSkillUninstall(ctx context.Context, profile, name string) (string, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	var result skillHubActionResponse
	body := skillUninstallRequest{Name: name, Profile: profile}
	if err := c.requestJSON(ctx, http.MethodPost, "/api/skills/hub/uninstall", profileQuery(profile), body, &result); err != nil {
		return "", err
	}
	if !result.OK {
		if strings.TrimSpace(result.Error) == "" {
			return "", fmt.Errorf("hermes rejected skill uninstallation")
		}
		return "", fmt.Errorf("hermes rejected skill uninstallation: %s", result.Error)
	}
	if strings.TrimSpace(result.Name) == "" {
		return "", fmt.Errorf("hermes skill uninstallation response did not include an action name")
	}
	return result.Name, nil
}

// GetActionStatus reads only bounded action metadata. The request asks Hermes
// for one log line as an additional server-side safety limit; the response
// decoder still ignores the log field entirely.
func (c *Client) GetActionStatus(ctx context.Context, name string) (ActionStatus, error) {
	var result ActionStatus
	query := url.Values{"lines": []string{"1"}}.Encode()
	path := "/api/actions/" + url.PathEscape(name) + "/status"
	if err := c.requestJSON(ctx, http.MethodGet, path, query, nil, &result); err != nil {
		return ActionStatus{}, err
	}
	return result, nil
}

const actionPollInterval = 500 * time.Millisecond

// WaitForAction waits for a Hermes background action to finish. It never
// includes action log lines in errors and always observes the caller's
// context/deadline.
func (c *Client) WaitForAction(ctx context.Context, name string, timeout time.Duration) error {
	if timeout <= 0 {
		return fmt.Errorf("hermes action timeout must be positive")
	}
	pollContext, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	for {
		status, err := c.GetActionStatus(pollContext, name)
		if err != nil {
			return fmt.Errorf("read Hermes action %q status: %w", name, err)
		}
		if !status.Running {
			if status.ExitCode == nil {
				return fmt.Errorf("hermes action %q completed without an exit code", name)
			}
			if *status.ExitCode != 0 {
				return fmt.Errorf("hermes action %q failed with exit code %d", name, *status.ExitCode)
			}
			return nil
		}

		timer := time.NewTimer(actionPollInterval)
		select {
		case <-pollContext.Done():
			if err := pollContext.Err(); err != nil {
				return fmt.Errorf("wait for hermes action %q: %w", name, err)
			}
			return fmt.Errorf("wait for hermes action %q ended unexpectedly", name)
		case <-timer.C:
		}
	}
}
