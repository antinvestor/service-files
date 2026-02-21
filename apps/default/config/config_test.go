package config_test

import (
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type ConfigTestSuite struct {
	tests.BaseTestSuite
}

func TestConfigTestSuite(t *testing.T) {
	suite.Run(t, new(ConfigTestSuite))
}

func (s *ConfigTestSuite) TestFilesConfig_Normalise() {
	cases := []struct {
		name string
		cfg  config.FilesConfig
	}{
		{
			name: "defaults_applied",
			cfg:  config.FilesConfig{},
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg
			require.NoError(t, cfg.Normalise())
			require.NotZero(t, cfg.MaxFileSizeBytes)
			require.NotEmpty(t, cfg.AbsBasePath)
			require.NotZero(t, cfg.MaxThumbnailGenerators)
			require.NotEmpty(t, cfg.ThumbnailSizes)
			require.NotZero(t, cfg.MaxThumbnailDimension)
		})
	}
}
