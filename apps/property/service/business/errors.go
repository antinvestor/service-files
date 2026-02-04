package business

import (
	"connectrpc.com/connect"
)

var (
	ErrorUnspecifiedID      = connect.NewError(connect.CodeInvalidArgument, nil)
	ErrorEmptyValueSupplied = connect.NewError(connect.CodeInvalidArgument, nil)
	ErrorItemExist          = connect.NewError(connect.CodeAlreadyExists, nil)
	ErrorItemDoesNotExist   = connect.NewError(connect.CodeNotFound, nil)
	ErrorInitializationFail = connect.NewError(connect.CodeInternal, nil)
)
