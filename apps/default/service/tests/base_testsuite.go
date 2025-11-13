package tests

import (
	"context"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	events2 "github.com/antinvestor/service-files/apps/default/service/events"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	internaltests "github.com/antinvestor/service-files/internal/tests"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/frametests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/require"
)

type BaseTestSuite struct {
	internaltests.BaseTestSuite
}

func (bs *BaseTestSuite) CreateService(
	t *testing.T,
	depOpts *definition.DependancyOption,
) (*frame.Service, context.Context) {

	ctx := t.Context()
	profileConfig, err := frame.ConfigFromEnv[config.FilesConfig]()
	require.NoError(t, err)

	profileConfig.LogLevel = "debug"
	profileConfig.RunServiceSecurely = false
	profileConfig.ServerPort = ""

	res := depOpts.ByIsDatabase(ctx)
	testDS, cleanup, err0 := res.GetRandomisedDS(ctx, depOpts.Prefix())
	require.NoError(t, err0)

	t.Cleanup(func() {
		cleanup(ctx)
	})

	profileConfig.DatabasePrimaryURL = []string{testDS.String()}
	profileConfig.DatabaseReplicaURL = []string{testDS.String()}

	ctx, svc := frame.NewServiceWithContext(ctx, "profile tests",
		frame.WithConfig(&profileConfig),
		frame.WithDatastore(),
		frametests.WithNoopDriver())

	svc.Init(ctx, frame.WithRegisterEvents(
		events2.NewAuditSaveHandler(svc),
		events2.NewMetadataSaveHandler(svc)))

	err = repository.Migrate(ctx, svc, "../../migrations/0001")
	require.NoError(t, err)

	err = svc.Run(ctx, "")
	require.NoError(t, err)

	return svc, ctx
}
