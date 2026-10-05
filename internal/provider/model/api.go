package model

import (
	"context"

	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

// modelClient is deliberately narrow so resource tests can exercise drift
// behavior without coupling themselves to the HTTP transport.
type modelClient interface {
	GetConfig(context.Context, string) (hermes.DashboardConfig, error)
	SetModel(context.Context, hermes.ModelAssignment) error
}

type dashboardConfig = hermes.DashboardConfig
type modelAssignment = hermes.ModelAssignment
type modelAssignmentState = hermes.ModelAssignmentState
