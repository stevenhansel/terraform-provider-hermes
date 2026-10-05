package custom

import (
	"context"

	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

type customProviderClient interface {
	ListCustomEndpoints(context.Context, string) (hermes.CustomEndpointsResponse, error)
	UpsertCustomEndpoint(context.Context, string, hermes.CustomEndpointRequest) error
	DeleteCustomEndpoint(context.Context, string, string) error
}
