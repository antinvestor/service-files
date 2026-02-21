package testketo

import (
	"context"
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/stretchr/testify/suite"
)

type KetoDependencyTestSuite struct {
	tests.BaseTestSuite
}

func TestKetoDependencyTestSuite(t *testing.T) {
	suite.Run(t, new(KetoDependencyTestSuite))
}

func (suite *KetoDependencyTestSuite) TestNewWithOptsAndSetupValidation() {
	testCases := []struct {
		name      string
		run       func(t *testing.T) any
		validator func(t *testing.T, result any)
	}{
		{
			name: "new_with_opts_sets_expected_defaults",
			run: func(_ *testing.T) any {
				resource := NewWithOpts()
				dep, ok := resource.(*KetoDependency)
				if !ok {
					return nil
				}
				return dep
			},
			validator: func(t *testing.T, result any) {
				require.NotNil(t, result)
				dep := result.(*KetoDependency)
				assert.Equal(t, ImageName, dep.Name())
				assert.Contains(t, dep.Opts().NetworkAliases, "keto")
				assert.Contains(t, dep.Opts().Ports, "4467/tcp")
			},
		},
		{
			name: "setup_without_database_dependency_fails",
			run: func(_ *testing.T) any {
				dep := NewWithOpts().(*KetoDependency)
				return dep.Setup(context.Background(), nil)
			},
			validator: func(t *testing.T, result any) {
				err, ok := result.(error)
				require.True(t, ok)
				require.Error(t, err)
				assert.Contains(t, err.Error(), "no database dependency was supplied")
			},
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			out := tc.run(t)
			tc.validator(t, out)
		})
	}
}
