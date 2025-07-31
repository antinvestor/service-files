package tests

import (
	"context"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	events2 "github.com/antinvestor/service-files/apps/default/service/events"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	internaltests "github.com/antinvestor/service-files/internal/tests"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/tests/testdef"
	"github.com/stretchr/testify/require"
)

type BaseTestSuite struct {
	internaltests.BaseTestSuite
}

func (bs *BaseTestSuite) CreateService(
	t *testing.T,
	depOpts *testdef.DependancyOption,
) (*frame.Service, context.Context) {
	t.Setenv("OTEL_TRACES_EXPORTER", "none")
	profileConfig, err := frame.ConfigFromEnv[config.FilesConfig]()
	require.NoError(t, err)

	profileConfig.LogLevel = "debug"
	profileConfig.RunServiceSecurely = false
	profileConfig.ServerPort = ""

	for _, res := range depOpts.Database() {
		testDS, cleanup, err0 := res.GetRandomisedDS(t.Context(), depOpts.Prefix())
		require.NoError(t, err0)

		t.Cleanup(func() {
			cleanup(t.Context())
		})

		profileConfig.DatabasePrimaryURL = []string{testDS.String()}
		profileConfig.DatabaseReplicaURL = []string{testDS.String()}
	}

	ctx, svc := frame.NewServiceWithContext(t.Context(), "profile tests",
		frame.WithConfig(&profileConfig),
		frame.WithDatastore(),
		frame.WithNoopDriver())

	svc.Init(ctx, frame.WithRegisterEvents(
		events2.NewAuditSaveHandler(svc),
		events2.NewMetadataSaveHandler(svc)))

	err = repository.Migrate(ctx, svc, "../../migrations/0001")
	require.NoError(t, err)

	err = svc.Run(ctx, "")
	require.NoError(t, err)

	return svc, ctx
}
