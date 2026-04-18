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

	// LinkExpiredWebhooks is a comma-separated list of HTTP URLs to
	// POST when the liveness probe flips a link to EXPIRED. Each
	// subscriber receives a small JSON body and decides (by inspecting
	// affiliate_id) whether the signal is theirs. The primary consumer
	// today is stawi-jobs-candidates, which flips canonical_jobs.status
	// and emails saved-job bookmarkers.
	LinkExpiredWebhooks []string `env:"LINK_EXPIRED_WEBHOOKS" envSeparator:"," envDefault:""`
}
