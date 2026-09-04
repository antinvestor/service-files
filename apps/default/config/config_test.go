package config_test

import (
	"testing"

	"github.com/antinvestor/service-files/apps/default/config"
	"github.com/antinvestor/service-files/apps/default/service/tests"
	frameconfig "github.com/pitabwire/frame/v2/config"
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

func (s *ConfigTestSuite) TestParseThumbnailSizes() {
	cases := []struct {
		name    string
		spec    string
		want    []config.ThumbnailSize
		wantErr bool
	}{
		{
			name: "full_spec",
			spec: "32x32:crop,96x96:crop,640x480:scale",
			want: []config.ThumbnailSize{
				{Width: 32, Height: 32, ResizeMethod: "crop"},
				{Width: 96, Height: 96, ResizeMethod: "crop"},
				{Width: 640, Height: 480, ResizeMethod: "scale"},
			},
		},
		{
			name: "method_defaults_to_scale_and_whitespace_tolerated",
			spec: " 1200x630 , 400X400:CROP ,",
			want: []config.ThumbnailSize{
				{Width: 1200, Height: 630, ResizeMethod: "scale"},
				{Width: 400, Height: 400, ResizeMethod: "crop"},
			},
		},
		{name: "empty_spec", spec: "", want: nil},
		{name: "bad_method", spec: "32x32:stretch", wantErr: true},
		{name: "missing_height", spec: "32", wantErr: true},
		{name: "zero_width", spec: "0x32", wantErr: true},
		{name: "non_numeric", spec: "axb", wantErr: true},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			got, err := config.ParseThumbnailSizes(tc.spec)
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, got)
		})
	}
}

func (s *ConfigTestSuite) TestFilesConfig_NormaliseThumbnailSizes() {
	cases := []struct {
		name    string
		cfg     config.FilesConfig
		want    []config.ThumbnailSize
		wantErr bool
	}{
		{
			name: "unset_uses_defaults",
			cfg:  config.FilesConfig{},
			want: config.DefaultThumbnailSizes(),
		},
		{
			name: "spec_overrides_defaults",
			cfg:  config.FilesConfig{ThumbnailSizesSpec: "1200x630:scale"},
			want: []config.ThumbnailSize{{Width: 1200, Height: 630, ResizeMethod: "scale"}},
		},
		{
			name:    "spec_exceeding_max_dimension_rejected",
			cfg:     config.FilesConfig{ThumbnailSizesSpec: "4096x4096:scale", MaxThumbnailDimension: 2048},
			wantErr: true,
		},
		{
			name:    "malformed_spec_rejected",
			cfg:     config.FilesConfig{ThumbnailSizesSpec: "wide"},
			wantErr: true,
		},
	}

	for _, tc := range cases {
		s.T().Run(tc.name, func(t *testing.T) {
			cfg := tc.cfg
			err := cfg.Normalise()
			if tc.wantErr {
				require.Error(t, err)
				return
			}
			require.NoError(t, err)
			require.Equal(t, tc.want, cfg.ThumbnailSizes)
		})
	}
}

func (s *ConfigTestSuite) TestFilesConfig_EnvOverrides() {
	s.T().Setenv("THUMBNAIL_SIZES", "64x64:crop,1200x630")
	s.T().Setenv("DYNAMIC_THUMBNAILS", "true")
	s.T().Setenv("MAX_FILE_SIZE_BYTES", "52428800")

	cfg, err := frameconfig.FromEnv[config.FilesConfig]()
	require.NoError(s.T(), err)
	require.NoError(s.T(), cfg.Normalise())

	require.True(s.T(), cfg.DynamicThumbnails)
	require.Equal(s.T(), config.FileSizeBytes(52428800), cfg.MaxFileSizeBytes)
	require.Equal(s.T(), []config.ThumbnailSize{
		{Width: 64, Height: 64, ResizeMethod: "crop"},
		{Width: 1200, Height: 630, ResizeMethod: "scale"},
	}, cfg.ThumbnailSizes)
}
