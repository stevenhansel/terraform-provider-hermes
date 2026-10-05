package provider

import "github.com/stevenhansel/terraform-provider-hermes/internal/hermes"

// Client is retained as an alias while provider resources migrate to the
// domain packages. The implementation belongs to internal/hermes so future
// resources share one transport and session implementation.
type Client = hermes.Client

func NewClient(endpoint, username, password string) (*Client, error) {
	return hermes.NewClient(endpoint, username, password)
}

func NewClientWithVersion(endpoint, username, password, version string) (*Client, error) {
	return hermes.NewClientWithVersion(endpoint, username, password, version)
}
