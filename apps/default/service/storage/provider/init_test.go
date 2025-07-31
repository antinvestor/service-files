package provider

import (
	"context"
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/storage/provider/local"
	"github.com/antinvestor/service-files/internal/tests"
	"github.com/pitabwire/frame"
	"github.com/pitabwire/frame/tests/testdef"
	"github.com/stretchr/testify/suite"
)

type ProviderTestSuite struct {
	tests.BaseTestSuite
}

func TestProviderTestSuite(t *testing.T) {
	suite.Run(t, new(ProviderTestSuite))
}

func (suite *ProviderTestSuite) TestGetStorageProvider() {
	testCases := []struct {
		name          string
		expectedType  string
		shouldSucceed bool
	}{
		{
			name:          "should return local provider",
			expectedType:  "*local.ProviderLocal",
			shouldSucceed: true,
		},
	}

	suite.WithTestDependancies(suite.T(), func(t *testing.T, dep *testdef.DependancyOption) {
		for _, tc := range testCases {
			t.Run(tc.name, func(t *testing.T) {
				ctx := context.Background()

				cfg, err := frame.ConfigFromEnv[config.FilesConfig]()
				if err != nil {
					t.Errorf("Could not get file config : %v", err)
				}

				storageProvider, err := GetStorageProvider(ctx, &cfg)
				if !tc.shouldSucceed {
					if err == nil {
						t.Errorf("Expected error but got none")
					}
					return
				}

				if err != nil {
					t.Errorf("A file storageProvider should not have issues : %v", err)
				}

				_, ok := storageProvider.(*local.ProviderLocal)
				if !ok {
					t.Errorf("The storageProvider is supposed to be a local instance only")
				}
			})
		}
	})
}
