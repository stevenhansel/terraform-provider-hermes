package hermes

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
)

// CronJob is the stable subset of a Hermes cron job used by the provider.
// Hermes adds scheduler-maintained fields over time; Schedule and Deliver are
// raw values because their response representation differs between versions.
type CronJob struct {
	ID              string          `json:"id"`
	Profile         string          `json:"profile"`
	Name            string          `json:"name"`
	Prompt          string          `json:"prompt"`
	Schedule        json.RawMessage `json:"schedule"`
	ScheduleDisplay string          `json:"schedule_display"`
	Deliver         json.RawMessage `json:"deliver"`
	Skills          []string        `json:"skills"`
	Skill           string          `json:"skill"`
	Model           *string         `json:"model"`
	Provider        *string         `json:"provider"`
	BaseURL         *string         `json:"base_url"`
	Script          *string         `json:"script"`
	ContextFrom     []string        `json:"context_from"`
	EnabledToolsets []string        `json:"enabled_toolsets"`
	Workdir         *string         `json:"workdir"`
	NoAgent         bool            `json:"no_agent"`
	Paused          bool            `json:"paused"`
	Enabled         bool            `json:"enabled"`
	State           string          `json:"state"`
	NextRunAt       *string         `json:"next_run_at"`
	LastRunAt       *string         `json:"last_run_at"`
	LastStatus      *string         `json:"last_status"`
}

// ScheduleText returns the practitioner-facing schedule expression. Hermes
// stores a parsed schedule object and also exposes a display value; older
// records may contain only one of those representations.
func (j CronJob) ScheduleText() string {
	if value := stringOrTrimmed(j.ScheduleDisplay); value != "" {
		return value
	}
	return rawDisplayValue(j.Schedule)
}

// DeliverText returns the configured delivery target across Hermes response
// versions that represented it as either a string or a JSON value.
func (j CronJob) DeliverText() string {
	return rawDisplayValue(j.Deliver)
}

// SkillNames returns the canonical multi-skill field, falling back to the
// legacy single-skill field used by older Hermes jobs.
func (j CronJob) SkillNames() []string {
	if len(j.Skills) > 0 {
		return append([]string(nil), j.Skills...)
	}
	if value := stringOrTrimmed(j.Skill); value != "" {
		return []string{value}
	}
	return nil
}

// IsPaused derives the durable paused state. Current Hermes records encode it
// through state/enabled rather than a top-level paused field.
func (j CronJob) IsPaused() bool {
	return j.Paused || j.State == "paused"
}

// CronJobRequest is accepted by Hermes' create route.
type CronJobRequest struct {
	Paused          bool     `json:"paused"`
	PausedReason    string   `json:"paused_reason,omitempty"`
	Prompt          string   `json:"prompt,omitempty"`
	Schedule        string   `json:"schedule"`
	Name            string   `json:"name,omitempty"`
	Deliver         string   `json:"deliver,omitempty"`
	Skills          []string `json:"skills,omitempty"`
	Model           string   `json:"model,omitempty"`
	Provider        string   `json:"provider,omitempty"`
	BaseURL         string   `json:"base_url,omitempty"`
	Script          string   `json:"script,omitempty"`
	ContextFrom     []string `json:"context_from,omitempty"`
	EnabledToolsets []string `json:"enabled_toolsets,omitempty"`
	Workdir         string   `json:"workdir,omitempty"`
	NoAgent         bool     `json:"no_agent,omitempty"`
}

// ListCronJobs lists jobs for one profile. The dashboard's "all" aggregate
// is intentionally not used by profile-scoped resources.
func (c *Client) ListCronJobs(ctx context.Context, profile string) ([]CronJob, error) {
	var result []CronJob
	if err := c.requestJSON(ctx, http.MethodGet, "/api/cron/jobs", profileQuery(profile), nil, &result); err != nil {
		return nil, err
	}
	return result, nil
}

// GetCronJob reads one job by ID.
func (c *Client) GetCronJob(ctx context.Context, profile, id string) (CronJob, error) {
	var result CronJob
	path := "/api/cron/jobs/" + url.PathEscape(id)
	if err := c.requestJSON(ctx, http.MethodGet, path, profileQuery(profile), nil, &result); err != nil {
		return CronJob{}, err
	}
	return result, nil
}

// CreateCronJob creates a job and returns the server-assigned identity.
func (c *Client) CreateCronJob(ctx context.Context, profile string, request CronJobRequest) (CronJob, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	var result CronJob
	if err := c.requestJSON(ctx, http.MethodPost, "/api/cron/jobs", profileQuery(profile), request, &result); err != nil {
		return CronJob{}, err
	}
	return result, nil
}

// UpdateCronJob sends Hermes' partial update envelope.
func (c *Client) UpdateCronJob(ctx context.Context, profile, id string, updates map[string]any) (CronJob, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	var result CronJob
	body := struct {
		Updates map[string]any `json:"updates"`
	}{Updates: updates}
	path := "/api/cron/jobs/" + url.PathEscape(id)
	if err := c.requestJSON(ctx, http.MethodPut, path, profileQuery(profile), body, &result); err != nil {
		return CronJob{}, err
	}
	return result, nil
}

// DeleteCronJob removes a job by ID.
func (c *Client) DeleteCronJob(ctx context.Context, profile, id string) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	path := "/api/cron/jobs/" + url.PathEscape(id)
	return c.requestJSON(ctx, http.MethodDelete, path, profileQuery(profile), nil, nil)
}

// PauseCronJob pauses a job without deleting its durable definition.
func (c *Client) PauseCronJob(ctx context.Context, profile, id string) (CronJob, error) {
	return c.cronAction(ctx, profile, id, "pause")
}

// ResumeCronJob resumes a paused job.
func (c *Client) ResumeCronJob(ctx context.Context, profile, id string) (CronJob, error) {
	return c.cronAction(ctx, profile, id, "resume")
}

func (c *Client) cronAction(ctx context.Context, profile, id, action string) (CronJob, error) {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	var result CronJob
	path := "/api/cron/jobs/" + url.PathEscape(id) + "/" + action
	if err := c.requestJSON(ctx, http.MethodPost, path, profileQuery(profile), nil, &result); err != nil {
		return CronJob{}, err
	}
	return result, nil
}

func stringOrTrimmed(value string) string {
	return strings.TrimSpace(value)
}

func rawDisplayValue(value json.RawMessage) string {
	if len(value) == 0 || strings.TrimSpace(string(value)) == "null" {
		return ""
	}
	var text string
	if err := json.Unmarshal(value, &text); err == nil {
		return strings.TrimSpace(text)
	}
	var object map[string]any
	if err := json.Unmarshal(value, &object); err != nil {
		return ""
	}
	for _, key := range []string{"display", "value", "expr", "run_at"} {
		if text, ok := object[key].(string); ok && strings.TrimSpace(text) != "" {
			return strings.TrimSpace(text)
		}
	}
	return ""
}
