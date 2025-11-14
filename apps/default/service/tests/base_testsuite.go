package tests

import (
	"context"
	"testing"

	aconfig "github.com/antinvestor/service-files/apps/default/config"
	events2 "github.com/antinvestor/service-files/apps/default/service/events"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	internaltests "github.com/antinvestor/service-files/internal/tests"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/frametests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/require"
)

type ServiceResources struct {
	MediaRepository repository.MediaRepository
	AuditRepository repository.MediaAuditRepository
}

type BaseTestSuite struct {
	internaltests.BaseTestSuite
}

func (bs *BaseTestSuite) CreateService(t *testing.T, depOpts *definition.DependencyOption, ) (context.Context, *frame.Service, ServiceResources) {

	ctx := t.Context()
	profileConfig, err := config.FromEnv[aconfig.FilesConfig]()
	require.NoError(t, err)

	profileConfig.LogLevel = "debug"
	profileConfig.DatabaseMigrate = true
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

	ctx, svc := frame.NewServiceWithContext(ctx, frame.WithName("profile tests"),
		frame.WithConfig(&profileConfig),
		frame.WithDatastore(),
		frametests.WithNoopDriver())

	workMan := svc.WorkManager()
	dbManager := svc.DatastoreManager()
	dbPool := dbManager.GetPool(ctx, datastore.DefaultPoolName)

	deps := ServiceResources{
		MediaRepository: repository.NewMediaRepository(ctx, dbPool, workMan),
		AuditRepository: repository.NewMediaAuditRepository(ctx, dbPool, workMan),
	}

	svc.Init(ctx, frame.WithRegisterEvents(
		events2.NewAuditSaveHandler(deps.AuditRepository),
		events2.NewMetadataSaveHandler(deps.MediaRepository)))

	err = repository.Migrate(ctx, dbManager, "../../migrations/0001")
	require.NoError(t, err)

	err = svc.Run(ctx, "")
	require.NoError(t, err)

	return ctx, svc, deps
}
