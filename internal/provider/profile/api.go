package profile

import (
	"context"

	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

type profileClient interface {
	ListProfiles(context.Context) ([]hermes.Profile, error)
	CreateProfile(context.Context, hermes.CreateProfileRequest) error
	RenameProfile(context.Context, string, string) error
	UpdateProfileDescription(context.Context, string, string) error
	DeleteProfile(context.Context, string) error
}
