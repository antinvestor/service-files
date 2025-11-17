package routing

import (
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type RoutingTestSuite struct {
	tests.BaseTestSuite
}

func TestRoutingTestSuite(t *testing.T) {
	suite.Run(t, new(RoutingTestSuite))
}

func (suite *RoutingTestSuite) Test_IsValidMediaID() {
	testCases := []struct {
		name     string
		mediaID  string
		expected bool
	}{
		{
			name:     "valid media ID",
			mediaID:  "AbCdEf1234567890",
			expected: true,
		},
		{
			name:     "valid media ID with more characters",
			mediaID:  "validMediaID1234567890ABCDEFGHIJKLMNOPQRSTUVWXYZ",
			expected: true,
		},
		{
			name:     "invalid media ID with special chars",
			mediaID:  "invalid@id",
			expected: false,
		},
		{
			name:     "invalid media ID with spaces",
			mediaID:  "invalid id",
			expected: false,
		},
		{
			name:     "empty media ID",
			mediaID:  "",
			expected: false,
		},
		{
			name:     "short but valid media ID",
			mediaID:  "abc",
			expected: true, // Actually valid according to the validation function
		},
	}

	for _, tc := range testCases {
		t := suite.T()
		t.Run(tc.name, func(t *testing.T) {
			result := isValidMediaID(tc.mediaID)
			assert.Equal(t, tc.expected, result)
		})
	}
}
