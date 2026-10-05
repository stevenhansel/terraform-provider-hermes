package hermes

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
)

// DashboardConfig is the subset of Hermes configuration needed to discover
// model assignments. Unknown configuration fields are intentionally ignored.
type DashboardConfig struct {
	// The dashboard flattens model to a string; model/info retains its provider.
	MainProvider    string                 `json:"-"`
	Model           ModelConfig            `json:"model"`
	Auxiliary       AuxiliaryConfig        `json:"auxiliary"`
	CustomProviders []CustomProviderConfig `json:"custom_providers"`
}

// AuxiliaryConfig is intentionally more permissive than the model field.
// Hermes stores both model slots and auxiliary runtime settings (for example
// retry counts and feature flags) under the same auxiliary object. Only the
// object-shaped entries that look like model assignments are relevant to this
// provider; scalar metadata must not make an otherwise readable config fail.
type AuxiliaryConfig map[string]ModelConfig

func (a *AuxiliaryConfig) UnmarshalJSON(data []byte) error {
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		return fmt.Errorf("hermes auxiliary configuration must be an object: %w", err)
	}

	parsed := make(AuxiliaryConfig, len(raw))
	for key, value := range raw {
		trimmed := bytes.TrimSpace(value)
		if len(trimmed) == 0 {
			continue
		}
		if trimmed[0] != '"' && trimmed[0] != '{' {
			// Hermes also uses this namespace for scalar runtime settings.
			continue
		}

		var model ModelConfig
		if err := json.Unmarshal(value, &model); err != nil {
			return fmt.Errorf("decode Hermes auxiliary model %q: %w", key, err)
		}
		parsed[key] = model
	}
	*a = parsed
	return nil
}

// ModelConfig accepts both shapes returned by Hermes across versions:
// "model-name" and {"provider":"custom", "default":"model-name"}.
type ModelConfig struct {
	Raw      *string
	Provider *string
	Default  *string
	Model    *string
	Name     *string
	BaseURL  *string
	API      *string
}

func (m *ModelConfig) UnmarshalJSON(data []byte) error {
	*m = ModelConfig{}

	var raw string
	if err := json.Unmarshal(data, &raw); err == nil {
		m.Raw = &raw
		return nil
	}

	var object struct {
		Provider *string `json:"provider"`
		Default  *string `json:"default"`
		Model    *string `json:"model"`
		Name     *string `json:"name"`
		BaseURL  *string `json:"base_url"`
		API      *string `json:"api"`
	}
	if err := json.Unmarshal(data, &object); err != nil {
		return fmt.Errorf("hermes model configuration must be a string or object: %w", err)
	}
	*m = ModelConfig{
		Provider: object.Provider,
		Default:  object.Default,
		Model:    object.Model,
		Name:     object.Name,
		BaseURL:  object.BaseURL,
		API:      object.API,
	}
	return nil
}

// CustomProviderConfig describes a named custom provider returned by Hermes.
type CustomProviderConfig struct {
	Name    string `json:"name"`
	BaseURL string `json:"base_url"`
	API     string `json:"api"`
	Model   string `json:"model"`
}

// ModelAssignment is the request body accepted by Hermes' model-set route.
type ModelAssignment struct {
	Scope                 string `json:"scope"`
	Provider              string `json:"provider"`
	Model                 string `json:"model"`
	Task                  string `json:"task,omitempty"`
	BaseURL               string `json:"base_url,omitempty"`
	APIKey                string `json:"api_key,omitempty"`
	ConfirmExpensiveModel bool   `json:"confirm_expensive_model,omitempty"`
	Profile               string `json:"profile,omitempty"`
}

// ModelAssignmentResponse is the application-level response returned by
// POST /api/model/set. Hermes may return HTTP 200 with ok=false when a model
// selection needs explicit confirmation, so HTTP status alone is not enough
// to determine whether the mutation was applied.
type ModelAssignmentResponse struct {
	OK              *bool  `json:"ok"`
	ConfirmRequired bool   `json:"confirm_required"`
	ConfirmMessage  string `json:"confirm_message"`
	Message         string `json:"message"`
	Error           string `json:"error"`
}

// ModelAssignmentState is the normalized model assignment discovered from a
// dashboard config response. BaseURLKnown distinguishes an omitted URL from
// an explicitly empty URL so resources can preserve write-only/unknown state.
type ModelAssignmentState struct {
	Scope        string
	Task         string
	Provider     string
	Model        string
	BaseURL      string
	BaseURLKnown bool
}

// GetConfig reads the dashboard configuration for a profile.
func (c *Client) GetConfig(ctx context.Context, profile string) (DashboardConfig, error) {
	var config DashboardConfig
	if err := c.requestJSON(ctx, http.MethodGet, "/api/config", profileQuery(profile), nil, &config); err != nil {
		return DashboardConfig{}, err
	}
	if config.Model.Raw != nil && strings.TrimSpace(*config.Model.Raw) != "" {
		var info struct {
			Model    string `json:"model"`
			Provider string `json:"provider"`
		}
		if err := c.requestJSON(ctx, http.MethodGet, "/api/model/info", profileQuery(profile), nil, &info); err != nil {
			return DashboardConfig{}, fmt.Errorf("read flattened model provider: %w", err)
		}
		if strings.TrimSpace(info.Model) != strings.TrimSpace(*config.Model.Raw) || strings.TrimSpace(info.Provider) == "" {
			return DashboardConfig{}, fmt.Errorf("model configuration and model info disagree; retry after the model selection settles")
		}
		config.MainProvider = strings.TrimSpace(info.Provider)
	}
	return config, nil
}

// SetModel assigns a main or auxiliary model. Hermes does not expose a safe
// inverse operation, so callers must treat this as a mutation-only API.
func (c *Client) SetModel(ctx context.Context, assignment ModelAssignment) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	var result ModelAssignmentResponse
	if err := c.requestJSON(ctx, http.MethodPost, "/api/model/set", profileQuery(assignment.Profile), assignment, &result); err != nil {
		return err
	}
	if result.OK != nil && !*result.OK {
		message := result.ConfirmMessage
		if strings.TrimSpace(message) == "" {
			message = result.Message
		}
		if strings.TrimSpace(message) == "" {
			message = result.Error
		}
		return &OperationError{
			Operation:       "model assignment",
			Message:         message,
			ConfirmRequired: result.ConfirmRequired,
		}
	}
	return nil
}

// FindModelAssignment returns a normalized assignment for the requested
// scope. It returns false when Hermes has no usable assignment for that slot.
func (c DashboardConfig) FindModelAssignment(scope, task string) (ModelAssignmentState, bool) {
	if scope == "" {
		scope = "main"
	}

	switch scope {
	case "main":
		return c.mainModelAssignment()
	case "auxiliary":
		if strings.TrimSpace(task) == "" {
			return ModelAssignmentState{}, false
		}
		model, ok := c.Auxiliary[task]
		if !ok {
			return ModelAssignmentState{}, false
		}
		return modelAssignmentFromConfig("auxiliary", task, model)
	default:
		return ModelAssignmentState{}, false
	}
}

func (c DashboardConfig) mainModelAssignment() (ModelAssignmentState, bool) {
	if c.Model.Raw != nil {
		model := strings.TrimSpace(*c.Model.Raw)
		if model == "" {
			return ModelAssignmentState{}, false
		}
		baseURL, baseURLKnown := c.customModelDetails(model)
		provider := c.MainProvider
		if provider == "" {
			provider = "custom"
		}
		return ModelAssignmentState{
			Scope:        "main",
			Provider:     provider,
			Model:        model,
			BaseURL:      baseURL,
			BaseURLKnown: baseURLKnown,
		}, true
	}
	return modelAssignmentFromConfig("main", "", c.Model)
}

func modelAssignmentFromConfig(scope, task string, config ModelConfig) (ModelAssignmentState, bool) {
	model := firstModelString(config.Default, config.Model, config.Name)
	provider, providerKnown := nonEmptyString(config.Provider)
	if model == "" || !providerKnown {
		return ModelAssignmentState{}, false
	}
	baseURL, baseURLKnown := config.baseURL()
	return ModelAssignmentState{
		Scope:        scope,
		Task:         task,
		Provider:     provider,
		Model:        model,
		BaseURL:      baseURL,
		BaseURLKnown: baseURLKnown,
	}, true
}

func (m ModelConfig) baseURL() (string, bool) {
	if m.BaseURL != nil {
		return strings.TrimSpace(*m.BaseURL), true
	}
	if m.API != nil {
		return strings.TrimSpace(*m.API), true
	}
	return "", false
}

func (c DashboardConfig) customModelDetails(model string) (baseURL string, found bool) {
	for _, provider := range c.CustomProviders {
		if strings.TrimSpace(provider.Model) != strings.TrimSpace(model) {
			continue
		}
		if strings.TrimSpace(provider.BaseURL) != "" {
			return provider.BaseURL, true
		}
		if strings.TrimSpace(provider.API) != "" {
			return provider.API, true
		}
		return "", true
	}
	return "", false
}

func firstModelString(values ...*string) string {
	for _, value := range values {
		if value != nil {
			if normalized := strings.TrimSpace(*value); normalized != "" {
				return normalized
			}
		}
	}
	return ""
}

func nonEmptyString(value *string) (string, bool) {
	if value == nil {
		return "", false
	}
	valueString := strings.TrimSpace(*value)
	return valueString, valueString != ""
}
