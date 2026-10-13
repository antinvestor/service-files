package apierror_test

import (
	"context"
	"errors"
	"fmt"
	"testing"

	"connectrpc.com/connect/v2"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gorm.io/gorm"

	"github.com/antinvestor/service-files/internal/apierror"
)

func TestConvert(t *testing.T) {
	plain := errors.New("property type missing")
	built := connect.NewError(connect.CodePermissionDenied, "denied")

	tests := []struct {
		name    string
		err     error
		code    connect.Code
		message string
		same    bool
	}{
		{name: "nil", err: nil},
		{name: "connect error passes through", err: built, same: true},
		{name: "wrapped connect error passes through", err: fmt.Errorf("wrap: %w", built), same: true},
		{name: "context canceled passes through", err: context.Canceled, same: true},
		{name: "record not found maps to not found", err: fmt.Errorf("lookup: %w", gorm.ErrRecordNotFound), code: connect.CodeNotFound, message: "lookup: record not found"},
		{name: "plain error keeps its message", err: plain, code: connect.CodeInternal, message: "property type missing"},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			got := apierror.Convert(tc.err)
			if tc.err == nil {
				require.NoError(t, got)
				return
			}
			if tc.same {
				assert.Equal(t, tc.err, got)
				return
			}
			var cerr *connect.Error
			require.ErrorAs(t, got, &cerr)
			assert.Equal(t, tc.code, cerr.Code())
			assert.Equal(t, tc.message, cerr.Message())
			assert.ErrorIs(t, got, tc.err)
		})
	}
}
