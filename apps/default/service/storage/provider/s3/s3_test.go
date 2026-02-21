package s3

import (
	"context"
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type S3ProviderTestSuite struct {
	tests.BaseTestSuite
}

func TestS3ProviderTestSuite(t *testing.T) {
	suite.Run(t, new(S3ProviderTestSuite))
}

func (suite *S3ProviderTestSuite) TestNewProviderAndSetup() {
	testCases := []struct {
		name          string
		privateBucket string
		publicBucket  string
	}{
		{
			name:          "creates_and_sets_up_provider",
			privateBucket: "s3-private",
			publicBucket:  "s3-public",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			p := NewProvider(
				"S3",
				tc.privateBucket,
				tc.publicBucket,
				"http://localhost:9000",
				"us-east-1",
				"secret",
				"token",
				"access",
			)
			require.NotNil(t, p)
			assert.Equal(t, "S3", p.Name())
			assert.Equal(t, tc.privateBucket, p.GetBucket(false))
			assert.Equal(t, tc.publicBucket, p.GetBucket(true))

			require.NoError(t, p.Setup(context.Background()))
		})
	}
}

func (suite *S3ProviderTestSuite) TestInit() {
	testCases := []struct {
		name      string
		setup     bool
		expectErr bool
	}{
		{
			name:      "init_without_setup_fails",
			setup:     false,
			expectErr: true,
		},
		{
			name:      "init_with_setup_attempts_open",
			setup:     true,
			expectErr: false,
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			p := NewProvider(
				"S3",
				"s3-private",
				"s3-public",
				"http://localhost:9000",
				"us-east-1",
				"secret",
				"token",
				"access",
			)
			require.NotNil(t, p)

			if tc.setup {
				require.NoError(t, p.Setup(context.Background()))
			}

			bucket, err := p.Init(context.Background(), "bucket")
			if tc.expectErr {
				require.Error(t, err)
				require.Nil(t, bucket)
				return
			}
			require.NoError(t, err)
			require.NotNil(t, bucket)
			require.NoError(t, bucket.Close())
		})
	}
}
