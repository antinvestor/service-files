package tests

import (
	"context"
	"testing"

	aconfig "github.com/antinvestor/service-files/apps/default/config"
	events2 "github.com/antinvestor/service-files/apps/default/service/events"
	"github.com/antinvestor/service-files/apps/default/service/storage/repository"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/datastore"
	"github.com/pitabwire/frame/frametests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/pitabwire/frame/frametests/deps/testpostgres"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/require"
)

type ServiceResources struct {
	MediaRepository         repository.MediaRepository
	AuditRepository         repository.MediaAuditRepository
	MultipartUploadRepo     repository.MultipartUploadRepository
	MultipartUploadPartRepo repository.MultipartUploadPartRepository
	FileVersionRepo         repository.FileVersionRepository
	RetentionPolicyRepo     repository.RetentionPolicyRepository
	FileRetentionRepo       repository.FileRetentionRepository
	StorageStatsRepo        repository.StorageStatsRepository
}

type BaseTestSuite struct {
	frametests.FrameBaseTestSuite
}

func initResources(_ context.Context) []definition.TestResource {
	pg := testpostgres.NewWithOpts("service_test",
		definition.WithUserName("test"),
		definition.WithEnableLogging(true),
		definition.WithEnvironment(map[string]string{
			"POSTGRES_MAX_CONNECTIONS": "500",
		}))

	return []definition.TestResource{pg}
}

func (bs *BaseTestSuite) SetupSuite() {
	if bs.InitResourceFunc == nil {
		bs.InitResourceFunc = initResources
	}
	bs.FrameBaseTestSuite.SetupSuite()
}

func (bs *BaseTestSuite) CreateService(t *testing.T, depOpts *definition.DependencyOption) (context.Context, *frame.Service, ServiceResources) {

	ctx := t.Context()
	profileConfig, err := config.FromEnv[aconfig.FilesConfig]()
	require.NoError(t, err)

	profileConfig.LogLevel = "debug"
	profileConfig.DatabaseMigrate = true
	profileConfig.RunServiceSecurely = false
	profileConfig.ServerPort = ""
	profileConfig.EnvStorageEncryptionPhrase = "0123456789abcdef0123456789abcdef"
	profileConfig.BasePath = aconfig.Path(t.TempDir())

	err = profileConfig.Normalise()
	require.NoError(t, err)

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
		MediaRepository:         repository.NewMediaRepository(ctx, dbPool, workMan),
		AuditRepository:         repository.NewMediaAuditRepository(ctx, dbPool, workMan),
		MultipartUploadRepo:     repository.NewMultipartUploadRepository(ctx, dbPool),
		MultipartUploadPartRepo: repository.NewMultipartUploadPartRepository(ctx, dbPool),
		FileVersionRepo:         repository.NewFileVersionRepository(ctx, dbPool),
		RetentionPolicyRepo:     repository.NewRetentionPolicyRepository(ctx, dbPool),
		FileRetentionRepo:       repository.NewFileRetentionRepository(ctx, dbPool),
		StorageStatsRepo:        repository.NewStorageStatsRepository(ctx, dbPool),
	}

	svc.Init(ctx, frame.WithRegisterEvents(
		events2.NewAuditSaveHandler(deps.AuditRepository),
		events2.NewMetadataSaveHandler(deps.MediaRepository)))

	err = repository.Migrate(ctx, dbManager, "../../migrations/0001")
	require.NoError(t, err)

	err = svc.Run(ctx, "")
	require.NoError(t, err)

	t.Cleanup(func() {
		svc.Stop(ctx)
	})

	return ctx, svc, deps
}

// WithTestDependancies creates subtests with each known DependencyOption.
func (bs *BaseTestSuite) WithTestDependancies(t *testing.T, testFn func(t *testing.T, dep *definition.DependencyOption)) {
	options := []*definition.DependencyOption{
		definition.NewDependancyOption("default", util.RandomAlphaNumericString(8), bs.Resources()),
	}
	frametests.WithTestDependencies(t, options, testFn)
}
