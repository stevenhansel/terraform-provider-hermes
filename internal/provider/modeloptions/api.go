package modeloptions

import (
	"context"

	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

type modelOptionsClient interface {
	GetModelOptions(context.Context, string, bool, bool, bool) (hermes.ModelOptions, error)
}
