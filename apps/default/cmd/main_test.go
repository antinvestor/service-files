package main

import (
	"testing"

	aconfig "github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/config"
	"github.com/pitabwire/frame/frametests"
	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type MainTestSuite struct {
	tests.BaseTestSuite
}

func TestMainTestSuite(t *testing.T) {
	suite.Run(t, new(MainTestSuite))
}

func (suite *MainTestSuite) TestValidateEncryptionConfig() {
	testCases := []struct {
		name      string
		phrase    string
		shouldErr bool
	}{
		{
			name:      "empty_phrase",
			phrase:    "",
			shouldErr: true,
		},
		{
			name:      "short_phrase",
			phrase:    "short",
			shouldErr: true,
		},
		{
			name:      "valid_32_byte_phrase",
			phrase:    "0123456789abcdef0123456789abcdef",
			shouldErr: false,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			cfg := &aconfig.FilesConfig{EnvStorageEncryptionPhrase: tc.phrase}
			err := validateEncryptionConfig(cfg)
			if tc.shouldErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
		})
	}
}

func (suite *MainTestSuite) TestHandleDatabaseMigration() {
	testCases := []struct {
		name     string
		migrate  bool
		wantTrue bool
	}{
		{
			name:     "no_migration_requested",
			migrate:  false,
			wantTrue: false,
		},
		{
			name:     "migration_requested",
			migrate:  true,
			wantTrue: true,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				if !tc.migrate {
					ctx, svc, _ := suite.CreateService(t, dep)
					cfg := *(svc.Config().(*aconfig.FilesConfig))
					cfg.DatabaseMigrate = tc.migrate
					cfg.DatabaseMigrationPath = "apps/default/migrations/0001"

					result := handleDatabaseMigration(ctx, svc.DatastoreManager(), cfg)
					assert.Equal(t, tc.wantTrue, result)
					return
				}

				// Migration must run on a privileged connection. The suite's
				// CreateService drops application connections to the
				// unprivileged rlstest role once migration completes, so
				// re-running migrations through that service would (rightly)
				// fail with permission errors. Use a dedicated service over a
				// fresh database instead, mirroring production where the
				// migration runner owns the schema.
				ctx := t.Context()
				cfg, err := config.FromEnv[aconfig.FilesConfig]()
				require.NoError(t, err)
				cfg.DatabaseMigrate = true
				cfg.DatabaseMigrationPath = "../migrations/0001"
				cfg.EnvStorageEncryptionPhrase = "0123456789abcdef0123456789abcdef"
				cfg.BasePath = aconfig.Path(t.TempDir())
				require.NoError(t, cfg.Normalise())

				res := dep.ByIsDatabase(ctx)
				testDS, cleanup, err0 := res.GetRandomisedDS(ctx, dep.Prefix()+"mig")
				require.NoError(t, err0)
				t.Cleanup(func() { cleanup(ctx) })
				cfg.DatabasePrimaryURL = []string{testDS.String()}
				cfg.DatabaseReplicaURL = []string{}

				ctx, svc := frame.NewServiceWithContext(ctx, frame.WithName("migration tests"),
					frame.WithConfig(&cfg),
					frame.WithDatastore(),
					frametests.WithNoopDriver())
				t.Cleanup(func() { svc.Stop(ctx) })
				svc.Init(ctx)

				result := handleDatabaseMigration(ctx, svc.DatastoreManager(), cfg)
				assert.Equal(t, tc.wantTrue, result)
			})
		}
	})
}
