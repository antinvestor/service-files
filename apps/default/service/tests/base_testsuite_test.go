package tests

import (
	"context"
	"testing"

	"github.com/pitabwire/frame/frametests/definition"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type BaseTestSuiteCoverageSuite struct {
	BaseTestSuite
}

func TestBaseTestSuiteCoverageSuite(t *testing.T) {
	suite.Run(t, new(BaseTestSuiteCoverageSuite))
}

func (suite *BaseTestSuiteCoverageSuite) TestInitResourcesAndWithDependencies() {
	testCases := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "init_resources_returns_default_postgres_dependency",
			run: func(t *testing.T) {
				resources := initResources(context.Background())
				require.Len(t, resources, 1)
				assert.NotNil(t, resources[0])
			},
		},
		{
			name: "with_test_dependencies_invokes_callback",
			run: func(t *testing.T) {
				callCount := 0
				suite.WithTestDependancies(t, func(_ *testing.T, dep *definition.DependencyOption) {
					require.NotNil(t, dep)
					callCount++
				})
				assert.Equal(t, 1, callCount)
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			tc.run(t)
		})
	}
}

func (suite *BaseTestSuiteCoverageSuite) TestCreateService() {
	testCases := []struct {
		name string
	}{
		{name: "create_service_with_repositories"},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *definition.DependencyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx, svc, resources := suite.CreateService(t, dep)
				require.NotNil(t, ctx)
				require.NotNil(t, svc)
				require.NotNil(t, resources.MediaRepository)
				require.NotNil(t, resources.AuditRepository)
			})
		}
	})
}
