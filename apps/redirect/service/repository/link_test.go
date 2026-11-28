package repository_test

import (
	"context"
	"testing"
	"time"

	"github.com/antinvestor/service-files/apps/redirect/config"
	"github.com/antinvestor/service-files/apps/redirect/service/models"
	"github.com/antinvestor/service-files/apps/redirect/service/repository"
	"github.com/pitabwire/frame/v3"
	fconfig "github.com/pitabwire/frame/v3/config"
	"github.com/pitabwire/frame/v3/datastore"
	"github.com/pitabwire/frame/v3/frametests"
	"github.com/pitabwire/frame/v3/frametests/definition"
	"github.com/pitabwire/frame/v3/frametests/deps/testpostgres"
	"github.com/pitabwire/util"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
	"gorm.io/gorm"
)

type LinkRepositoryTestSuite struct {
	frametests.FrameBaseTestSuite
}

func TestLinkRepositoryTestSuite(t *testing.T) {
	suite.Run(t, new(LinkRepositoryTestSuite))
}

func (s *LinkRepositoryTestSuite) SetupSuite() {
	s.InitResourceFunc = func(_ context.Context) []definition.TestResource {
		return []definition.TestResource{
			testpostgres.NewWithOpts("redirect_test", definition.WithUserName("test")),
		}
	}
	s.FrameBaseTestSuite.SetupSuite()
}

// newRepo starts a Frame service on a randomised database with the redirect
// schema migrated, mirroring what the setup Job does in production.
func (s *LinkRepositoryTestSuite) newRepo(t *testing.T) (context.Context, repository.LinkRepository) {
	t.Helper()
	dep := definition.NewDependancyOption("default", util.RandomAlphaNumericString(8), s.Resources())

	ctx := t.Context()
	cfg, err := fconfig.FromEnv[config.RedirectConfig]()
	require.NoError(t, err)
	cfg.ServerPort = ""
	cfg.DatabaseMigrate = true
	cfg.DatabaseMaxOpenConnections = 2
	cfg.DatabaseMaxIdleConnections = 0
	cfg.AuthorizationMode = fconfig.AuthorizationModeDisabled

	testDS, cleanup, err := dep.ByIsDatabase(ctx).GetRandomisedDS(ctx, dep.Prefix())
	require.NoError(t, err)
	t.Cleanup(func() { cleanup(ctx) })
	cfg.DatabasePrimaryURL = []string{testDS.String()}
	cfg.DatabaseReplicaURL = []string{}

	ctx, svc := frame.NewServiceWithContext(ctx, frame.WithName("redirect repo tests"),
		frame.WithConfig(&cfg), frame.WithDatastore(), frametests.WithNoopDriver())
	t.Cleanup(func() { svc.Stop(ctx) })

	dbManager := svc.DatastoreManager()
	dbPool := dbManager.GetPool(ctx, datastore.DefaultPoolName)
	migrationPool := dbManager.GetPool(ctx, datastore.DefaultMigrationPoolName)
	require.NotNil(t, migrationPool)
	require.NoError(t, dbManager.Migrate(ctx, migrationPool, "../../migrations/0001", models.Link{}, models.Click{}))

	return ctx, repository.NewLinkRepository(dbPool)
}

func createLink(ctx context.Context, t *testing.T, repo repository.LinkRepository) *models.Link {
	t.Helper()
	link := &models.Link{
		Slug:           "s" + util.RandomAlphaNumericString(10),
		DestinationURL: "https://example.com/" + util.IDString(),
		State:          models.LinkStateActive,
	}
	require.NoError(t, repo.Create(ctx, link))
	require.NotEmpty(t, link.GetID())
	return link
}

func (s *LinkRepositoryTestSuite) TestStateTransitions_PersistAndBumpVersion() {
	testCases := []struct {
		name      string
		transit   func(ctx context.Context, repo repository.LinkRepository, id string) error
		wantState models.LinkState
	}{
		{name: "soft_delete", wantState: models.LinkStateDeleted,
			transit: func(ctx context.Context, r repository.LinkRepository, id string) error { return r.SoftDelete(ctx, id) }},
		{name: "expire", wantState: models.LinkStateExpired,
			transit: func(ctx context.Context, r repository.LinkRepository, id string) error { return r.ExpireLink(ctx, id) }},
	}

	ctx, repo := s.newRepo(s.T())
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			t := s.T()
			link := createLink(ctx, t, repo)

			require.NoError(t, tc.transit(ctx, repo, link.GetID()))

			got, err := repo.GetByID(ctx, link.GetID())
			require.NoError(t, err)
			require.Equal(t, tc.wantState, got.State, "state change must persist on the target row")
			require.Equal(t, link.Version+1, got.Version, "state change must advance the optimistic-lock version")
			require.True(t, got.ModifiedAt.After(link.ModifiedAt), "modified_at must advance")
			require.Equal(t, link.GetID(), got.GetID())

			err = tc.transit(ctx, repo, "unknown-"+util.IDString())
			require.ErrorIs(t, err, gorm.ErrRecordNotFound)
		})
	}
}

func (s *LinkRepositoryTestSuite) TestIncrementClickCount_AccumulatesOnTargetRow() {
	t := s.T()
	ctx, repo := s.newRepo(t)
	link := createLink(ctx, t, repo)
	other := createLink(ctx, t, repo)

	require.NoError(t, repo.IncrementClickCount(ctx, link.GetID(), 3, 1))
	require.NoError(t, repo.IncrementClickCount(ctx, link.GetID(), 2, 0))

	got, err := repo.GetByID(ctx, link.GetID())
	require.NoError(t, err)
	require.EqualValues(t, 5, got.ClickCount)
	require.EqualValues(t, 1, got.UniqueClickCount)
	require.Equal(t, link.Version, got.Version, "counters must not invalidate concurrent admin edits")

	untouched, err := repo.GetByID(ctx, other.GetID())
	require.NoError(t, err)
	require.Zero(t, untouched.ClickCount)

	require.ErrorIs(t, repo.IncrementClickCount(ctx, "unknown-"+util.IDString(), 1, 1), gorm.ErrRecordNotFound)
}

func (s *LinkRepositoryTestSuite) TestRecordCheck_PersistsProbeOutcome() {
	testCases := []struct {
		name     string
		status   int
		failures int
	}{
		{name: "failure", status: 404, failures: 2},
		{name: "recovery", status: 200, failures: 0},
	}

	ctx, repo := s.newRepo(s.T())
	link := createLink(ctx, s.T(), repo)
	for _, tc := range testCases {
		s.Run(tc.name, func() {
			t := s.T()
			before := time.Now().UTC().Add(-time.Second)

			require.NoError(t, repo.RecordCheck(ctx, link.GetID(), tc.status, tc.failures))

			got, err := repo.GetByID(ctx, link.GetID())
			require.NoError(t, err)
			require.Equal(t, tc.status, got.LastCheckStatus)
			require.Equal(t, tc.failures, got.ConsecutiveCheckFailures)
			require.True(t, got.LastCheckedAt.After(before), "last_checked_at must be recorded")
		})
	}

	require.ErrorIs(s.T(), repo.RecordCheck(ctx, "unknown-"+util.IDString(), 200, 0), gorm.ErrRecordNotFound)
}
