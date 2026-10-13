// Package apierror keeps handler errors visible to Connect clients.
package apierror

import (
	"context"
	"errors"

	"connectrpc.com/connect/v2"
	"github.com/pitabwire/frame/v3/data"
)

// ServerInterceptor converts a plain error returned by a handler into a
// *connect.Error through frame's data.ErrorConvertToAPI.
//
// connect v2 sends only errors built as *connect.Error; any other error reaches
// the client as CodeUnknown with no message. connect v1 sent err.Error(). This
// interceptor keeps the message and maps database errors to their codes.
// *connect.Error values and context errors pass through unchanged.
func ServerInterceptor() connect.ServerInterceptor {
	return func(next connect.ServerFunc) connect.ServerFunc {
		return func(ctx context.Context, spec connect.Spec, stream connect.ServerStream) error {
			return Convert(next(ctx, spec, stream))
		}
	}
}

// Convert returns err as a *connect.Error, leaving *connect.Error values and
// context errors unchanged.
func Convert(err error) error {
	if err == nil {
		return nil
	}
	var cerr *connect.Error
	if errors.As(err, &cerr) || errors.Is(err, context.Canceled) || errors.Is(err, context.DeadlineExceeded) {
		return err
	}
	return data.ErrorConvertToAPI(err)
}
