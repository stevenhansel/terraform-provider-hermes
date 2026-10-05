package status

import (
	"context"

	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

type statusClient interface {
	GetStatus(context.Context, string) (hermes.Status, error)
}
