package plugin

import (
	"context"

	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

type pluginClient interface {
	ListAgentPlugins(context.Context) ([]hermes.AgentPlugin, error)
	InstallAgentPlugin(context.Context, hermes.AgentPluginInstallRequest) (hermes.AgentPluginMutationResult, error)
	SetAgentPluginEnabled(context.Context, string, bool) (hermes.AgentPluginMutationResult, error)
	DeleteAgentPlugin(context.Context, string) (hermes.AgentPluginMutationResult, error)
}
