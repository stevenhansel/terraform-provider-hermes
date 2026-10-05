package skillinstallation

import (
	"context"
	"time"

	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

type skillInstallationClient interface {
	ListSkillHubSources(context.Context, string) (hermes.SkillHubSources, error)
	StartSkillInstall(context.Context, string, string) (string, error)
	StartSkillUninstall(context.Context, string, string) (string, error)
	WaitForAction(context.Context, string, time.Duration) error
}
