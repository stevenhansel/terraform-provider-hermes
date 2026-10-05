package hermes

import (
	"context"
	"net/http"
	"net/url"
)

// Profile is the stable, non-secret portion of a Hermes profile listing.
type Profile struct {
	Name             string  `json:"name"`
	Path             string  `json:"path"`
	IsDefault        bool    `json:"is_default"`
	Model            *string `json:"model"`
	Provider         *string `json:"provider"`
	HasEnv           bool    `json:"has_env"`
	SkillCount       int     `json:"skill_count"`
	GatewayRunning   bool    `json:"gateway_running"`
	Description      string  `json:"description"`
	DescriptionAuto  bool    `json:"description_auto"`
	DisplayName      string  `json:"display_name"`
	DistributionName *string `json:"distribution_name"`
	DistributionVer  *string `json:"distribution_version"`
	DistributionSrc  *string `json:"distribution_source"`
	HasAlias         bool    `json:"has_alias"`
}

type profileListResponse struct {
	Profiles []Profile `json:"profiles"`
}

// CreateProfileRequest contains the supported durable create options. The
// profile builder has additional best-effort fields; they are intentionally
// not exposed until their ownership and failure semantics are stable.
type CreateProfileRequest struct {
	Name             string `json:"name"`
	CloneFrom        string `json:"clone_from,omitempty"`
	CloneFromDefault bool   `json:"clone_from_default,omitempty"`
	CloneAll         bool   `json:"clone_all,omitempty"`
	NoSkills         bool   `json:"no_skills,omitempty"`
	Description      string `json:"description,omitempty"`
}

// ListProfiles returns all profiles visible to the authenticated dashboard
// session.
func (c *Client) ListProfiles(ctx context.Context) ([]Profile, error) {
	var result profileListResponse
	if err := c.requestJSON(ctx, http.MethodGet, "/api/profiles", "", nil, &result); err != nil {
		return nil, err
	}
	return result.Profiles, nil
}

// CreateProfile creates a named Hermes profile.
func (c *Client) CreateProfile(ctx context.Context, profile CreateProfileRequest) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return c.requestJSON(ctx, http.MethodPost, "/api/profiles", "", profile, nil)
}

// RenameProfile changes a named profile's identity. Hermes keeps the default
// profile's canonical name and treats its new name as display metadata; the
// provider protects the default profile before calling this method.
func (c *Client) RenameProfile(ctx context.Context, name, newName string) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	body := struct {
		NewName string `json:"new_name"`
	}{NewName: newName}
	return c.requestJSON(ctx, http.MethodPatch, profilePath(name), "", body, nil)
}

// UpdateProfileDescription updates only a profile's description.
func (c *Client) UpdateProfileDescription(ctx context.Context, name, description string) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	body := struct {
		Description string `json:"description"`
	}{Description: description}
	return c.requestJSON(ctx, http.MethodPut, profilePath(name)+"/description", "", body, nil)
}

// DeleteProfile permanently deletes a named profile. Callers should reject
// the default profile before invoking this method.
func (c *Client) DeleteProfile(ctx context.Context, name string) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return c.requestJSON(ctx, http.MethodDelete, profilePath(name), "", nil, nil)
}

func profilePath(name string) string {
	return "/api/profiles/" + url.PathEscape(name)
}
