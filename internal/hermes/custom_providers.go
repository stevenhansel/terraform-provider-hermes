package hermes

import (
	"context"
	"net/http"
	"net/url"
)

// CustomEndpoint is the secret-free representation returned by Hermes for a
// named OpenAI-compatible provider entry. API keys are represented only by
// HasAPIKey; the actual value is intentionally never decoded into provider
// state.
type CustomEndpoint struct {
	ID             string   `json:"id"`
	Name           string   `json:"name"`
	BaseURL        string   `json:"base_url"`
	Model          string   `json:"model"`
	Models         []string `json:"models"`
	ContextLength  *int     `json:"context_length"`
	DiscoverModels bool     `json:"discover_models"`
	HasAPIKey      bool     `json:"has_api_key"`
	IsCurrent      bool     `json:"is_current"`
	Source         string   `json:"source"`
}

// CustomEndpointsResponse is returned by Hermes' custom-endpoint listing and
// mutation routes. Current contains only the active model selection and is
// intentionally not managed by this resource.
type CustomEndpointsResponse struct {
	Endpoints []CustomEndpoint `json:"endpoints"`
	Current   struct {
		Provider string `json:"provider"`
		Model    string `json:"model"`
		BaseURL  string `json:"base_url"`
	} `json:"current"`
	OK bool   `json:"ok"`
	ID string `json:"id"`
}

// CustomEndpointRequest is the non-secret upsert contract for a named
// providers.<id> entry. APIKey is omitted by this provider; Hermes' endpoint
// supports external environment-backed credentials but cannot safely expose
// them through a Terraform refresh.
type CustomEndpointRequest struct {
	ID             string   `json:"id,omitempty"`
	Name           string   `json:"name"`
	BaseURL        string   `json:"base_url"`
	Model          string   `json:"model"`
	ContextLength  *int     `json:"context_length,omitempty"`
	DiscoverModels bool     `json:"discover_models"`
	Models         []string `json:"models,omitempty"`
}

// ListCustomEndpoints lists named custom endpoint entries for a profile.
func (c *Client) ListCustomEndpoints(ctx context.Context, profile string) (CustomEndpointsResponse, error) {
	var result CustomEndpointsResponse
	if err := c.requestJSON(ctx, http.MethodGet, "/api/providers/custom-endpoints", profileQuery(profile), nil, &result); err != nil {
		return CustomEndpointsResponse{}, err
	}
	return result, nil
}

// UpsertCustomEndpoint creates or updates one named custom endpoint.
func (c *Client) UpsertCustomEndpoint(ctx context.Context, profile string, endpoint CustomEndpointRequest) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return c.requestJSON(ctx, http.MethodPost, "/api/providers/custom-endpoints", profileQuery(profile), endpoint, nil)
}

// DeleteCustomEndpoint removes one named custom endpoint. Hermes also removes
// its generated environment-key reference and detaches it from the active
// main model when necessary.
func (c *Client) DeleteCustomEndpoint(ctx context.Context, profile, id string) error {
	c.writeMu.Lock()
	defer c.writeMu.Unlock()

	return c.requestJSON(ctx, http.MethodDelete, "/api/providers/custom-endpoints/"+url.PathEscape(id), profileQuery(profile), nil, nil)
}
