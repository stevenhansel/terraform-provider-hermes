package hermes

import (
	"context"
	"net/http"
	"net/url"
)

// ModelOptionProvider is the stable, non-secret subset of a provider row from
// GET /api/model/options. Hermes may add picker-only fields over time; the
// provider intentionally does not persist those fields in Terraform state.
type ModelOptionProvider struct {
	Slug          string   `json:"slug"`
	Name          string   `json:"name"`
	Models        []string `json:"models"`
	TotalModels   int      `json:"total_models"`
	IsCurrent     bool     `json:"is_current"`
	IsUserDefined bool     `json:"is_user_defined"`
	Source        string   `json:"source"`
	Authenticated bool     `json:"authenticated"`
	AuthType      string   `json:"auth_type"`
	KeyEnv        string   `json:"key_env"`
	Warning       string   `json:"warning"`
}

// ModelOptions is the stable response from Hermes' model picker inventory.
// Model and Provider identify the assignment currently saved in the selected
// profile; Providers contains the available provider/model rows.
type ModelOptions struct {
	Providers []ModelOptionProvider `json:"providers"`
	Model     string                `json:"model"`
	Provider  string                `json:"provider"`
}

// GetModelOptions reads the authenticated model inventory for a profile.
// refresh, includeUnconfigured, and explicitOnly map directly to Hermes'
// query parameters and are kept read-only: this endpoint does not mutate
// configuration.
func (c *Client) GetModelOptions(ctx context.Context, profile string, refresh, includeUnconfigured, explicitOnly bool) (ModelOptions, error) {
	values := url.Values{
		"refresh":              []string{boolQueryValue(refresh)},
		"include_unconfigured": []string{boolQueryValue(includeUnconfigured)},
		"explicit_only":        []string{boolQueryValue(explicitOnly)},
	}
	if profile != "" {
		values.Set("profile", profile)
	}

	var result ModelOptions
	if err := c.requestJSON(ctx, http.MethodGet, "/api/model/options", values.Encode(), nil, &result); err != nil {
		return ModelOptions{}, err
	}
	return result, nil
}

func boolQueryValue(value bool) string {
	if value {
		return "true"
	}
	return "false"
}
