package config

import (
	fconfig "github.com/pitabwire/frame/config"
)

type RedirectConfig struct {
	fconfig.ConfigurationDefault

	// OpenObserve ingest. Blank BaseURL keeps the handler silent —
	// local dev and preview envs don't need telemetry plumbing.
	AnalyticsBaseURL  string `env:"ANALYTICS_BASE_URL" envDefault:""`
	AnalyticsOrg      string `env:"ANALYTICS_ORG" envDefault:"default"`
	AnalyticsUsername string `env:"ANALYTICS_USERNAME" envDefault:""`
	AnalyticsPassword string `env:"ANALYTICS_PASSWORD" envDefault:""`

	// Dead-link CTA target — used on the /r/{slug} page we render
	// when the link is no longer active. Per-deploy because the jobs
	// service could in principle be accessible on a custom domain.
	JobsBaseURL string `env:"JOBS_BASE_URL" envDefault:"https://jobs.stawi.org/"`
}
