package service

import (
	"errors"
	"testing"

	"github.com/antinvestor/service-files/apps/default/service/tests"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type ServerTestSuite struct {
	tests.BaseTestSuite
}

func TestServerTestSuite(t *testing.T) {
	suite.Run(t, new(ServerTestSuite))
}

func (suite *ServerTestSuite) TestStatusError() {
	testCases := []struct {
		name     string
		code     int
		err      error
		expected string
	}{
		{
			name:     "returns_status_and_message",
			code:     404,
			err:      errors.New("not found"),
			expected: "not found",
		},
	}

	for _, tc := range testCases {
		suite.T().Run(tc.name, func(t *testing.T) {
			se := StatusError{Code: tc.code, Err: tc.err}
			assert.Equal(t, tc.expected, se.Error())
			assert.Equal(t, tc.code, se.Status())

			var wrapped Error = se
			assert.Equal(t, tc.code, wrapped.Status())
			assert.Equal(t, tc.expected, wrapped.Error())
		})
	}
}
