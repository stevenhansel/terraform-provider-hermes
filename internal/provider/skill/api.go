package skill

import (
	"context"

	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

type skillClient interface {
	ListSkills(context.Context, string) ([]hermes.Skill, error)
	SetSkillEnabled(context.Context, string, string, bool) error
}
