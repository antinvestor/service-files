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
	"github.com/pitabwire/frame/frametests/rlstest"
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
			"POSTGRES_MAX_CONNECTIONS": "300",
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
	// Migrate pins a dedicated connection for its advisory lock (frame
	// >= v1.95) and runs the migration queries on a second one, so a
	// single-connection pool deadlocks before the first table exists.
	profileConfig.DatabaseMaxOpenConnections = 2
	profileConfig.DatabaseMaxIdleConnections = 0
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
	profileConfig.DatabaseReplicaURL = []string{}

	// The postgres testcontainer user is a SUPERUSER which bypasses RLS even
	// with FORCE ROW LEVEL SECURITY, so tenancy isolation would never be
	// exercised. rlstest drops application connections to an unprivileged
	// role after migration so the suite runs with RLS actually enforced.
	require.NoError(t, rlstest.CreateRole(ctx, testDS.String()))
	rlsProv := rlstest.New()

	ctx, svc := frame.NewServiceWithContext(ctx, frame.WithName("profile tests"),
		frame.WithConfig(&profileConfig),
		frame.WithTenancyProvider(rlsProv),
		frame.WithDatastore(),
		frametests.WithNoopDriver())

	workMan := svc.WorkManager()
	dbManager := svc.DatastoreManager()
	dbPool := dbManager.GetPool(ctx, datastore.DefaultPoolName)

	deps := ServiceResources{
		MediaRepository:         repository.NewMediaRepository(ctx, dbPool, workMan),
		AuditRepository:         repository.NewMediaAuditRepository(ctx, dbPool, workMan),
		MultipartUploadRepo:     repository.NewMultipartUploadRepository(ctx, dbPool, workMan),
		MultipartUploadPartRepo: repository.NewMultipartUploadPartRepository(ctx, dbPool, workMan),
		FileVersionRepo:         repository.NewFileVersionRepository(ctx, dbPool, workMan),
		RetentionPolicyRepo:     repository.NewRetentionPolicyRepository(ctx, dbPool, workMan),
		FileRetentionRepo:       repository.NewFileRetentionRepository(ctx, dbPool, workMan),
		StorageStatsRepo:        repository.NewStorageStatsRepository(ctx, dbPool, workMan),
	}

	svc.Init(ctx, frame.WithRegisterEvents(
		events2.NewAuditSaveHandler(deps.AuditRepository),
		events2.NewMetadataSaveHandler(deps.MediaRepository)))

	err = repository.Migrate(ctx, dbManager, "../../migrations/0001")
	require.NoError(t, err)

	// Migration ran as superuser; grant the restricted role access to the
	// migrated tables, then switch all application queries to it.
	require.NoError(t, rlstest.GrantAll(ctx, testDS.String()))
	rlsProv.Enable()

	err = svc.Run(ctx, "")
	require.NoError(t, err)

	t.Cleanup(func() {
		svc.Stop(ctx)
		svc.DatastoreManager().Close(ctx)
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
