package business

import (
	"connectrpc.com/connect/v2"
)

var (
	ErrorUnspecifiedID      = connect.NewError(connect.CodeInvalidArgument, "")
	ErrorEmptyValueSupplied = connect.NewError(connect.CodeInvalidArgument, "")
	ErrorItemExist          = connect.NewError(connect.CodeAlreadyExists, "")
	ErrorItemDoesNotExist   = connect.NewError(connect.CodeNotFound, "")
	ErrorInitializationFail = connect.NewError(connect.CodeInternal, "")
)
