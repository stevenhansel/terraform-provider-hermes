package messaging

import (
	"context"

	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

type messagingClient interface {
	ListMessagingPlatforms(context.Context, string) ([]hermes.MessagingPlatform, error)
	UpdateMessagingPlatform(context.Context, string, string, hermes.MessagingPlatformUpdateRequest) error
}
