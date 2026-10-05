package cron

import (
	"context"

	"github.com/stevenhansel/terraform-provider-hermes/internal/hermes"
)

type cronClient interface {
	GetCronJob(context.Context, string, string) (hermes.CronJob, error)
	CreateCronJob(context.Context, string, hermes.CronJobRequest) (hermes.CronJob, error)
	UpdateCronJob(context.Context, string, string, map[string]any) (hermes.CronJob, error)
	DeleteCronJob(context.Context, string, string) error
	PauseCronJob(context.Context, string, string) (hermes.CronJob, error)
	ResumeCronJob(context.Context, string, string) (hermes.CronJob, error)
}
