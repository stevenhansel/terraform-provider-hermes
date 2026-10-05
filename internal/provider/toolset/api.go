package toolset

import (
	"context"

	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

type toolsetClient interface {
	ListToolsets(context.Context, string) ([]hermes.Toolset, error)
	SetToolsetEnabled(context.Context, string, string, bool) (hermes.ToolsetToggleResult, error)
}
