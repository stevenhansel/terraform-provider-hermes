package hermes

import (
	"context"
	"net/http"
)

// Status contains the stable, non-secret liveness fields exposed by Hermes'
// /api/status endpoint. The endpoint has additional version-specific fields;
// unknown fields are intentionally ignored by encoding/json.
type Status struct {
	Version          string   `json:"version"`
	ReleaseDate      string   `json:"release_date"`
	GatewayRunning   bool     `json:"gateway_running"`
	GatewayState     string   `json:"gateway_state"`
	ActiveAgents     int      `json:"active_agents"`
	ActiveSessions   int      `json:"active_sessions"`
	GatewayBusy      bool     `json:"gateway_busy"`
	GatewayDrainable bool     `json:"gateway_drainable"`
	AuthRequired     bool     `json:"auth_required"`
	AuthProviders    []string `json:"auth_providers"`
	AuthFlows        []string `json:"auth_flows"`
	Overall          string   `json:"overall"`
}

// GetStatus reads Hermes liveness and capability information. A non-empty
// profile is sent as the dashboard's profile query parameter.
func (c *Client) GetStatus(ctx context.Context, profile string) (Status, error) {
	var result Status
	if err := c.requestJSON(ctx, http.MethodGet, "/api/status", profileQuery(profile), nil, &result); err != nil {
		return Status{}, err
	}
	return result, nil
}
