package apperrors

import (
	"fmt"
	"strings"
)

const (
	// Non-retriable errors (400-499)
	BadRequest      = 400
	Unauthorised    = 401
	Forbidden       = 403
	NotFound        = 404
	Conflict        = 409
	Unprocessable   = 422
	TooManyRequests = 429

	// Retriable errors (500-599)
	InternalServerError = 500
	BadGateway          = 502
	ServiceUnavailable  = 503
	GatewayTimeout      = 504
)

// Predefined error instances
var (
	ErrSystemFailure = Error{
		code:    InternalServerError,
		message: "Internal system failure occurred",
	}
	ErrSystemTimeout = Error{
		code:    GatewayTimeout,
		message: "System operation timed out",
	}
	ErrResourceLimit = Error{
		code:    ServiceUnavailable,
		message: "System resource limit exceeded",
	}

	ErrIntegrationTimeout = Error{
		code:    GatewayTimeout,
		message: "Integration service timed out",
	}
	ErrIntegrationUnreachable = Error{
		code:    BadGateway,
		message: "Integration service unreachable",
	}
	ErrIntegrationRateLimit = Error{
		code:    TooManyRequests,
		message: "Integration service rate limit exceeded",
	}

	ErrInvalidInput = Error{
		code:    BadRequest,
		message: "Invalid input provided",
	}
	ErrMissingRequiredData = Error{
		code:    BadRequest,
		message: "Required data is missing",
	}
	ErrInvalidFormat = Error{
		code:    BadRequest,
		message: "Invalid data format",
	}

	ErrDataNotFound = Error{
		code:    NotFound,
		message: "Requested data not found",
	}
	ErrDataConflict = Error{
		code:    Conflict,
		message: "Data conflict occurred",
	}
	ErrDataValidation = Error{
		code:    Unprocessable,
		message: "Data validation failed",
	}

	ErrUnauthorizedAccess = Error{
		code:    Unauthorised,
		message: "Unauthorised access attempt",
	}
	ErrForbiddenAccess = Error{
		code:    Forbidden,
		message: "Access forbidden",
	}
	ErrInvalidCredentials = Error{
		code:    Unauthorised,
		message: "Invalid credentials provided",
	}
)

type Error struct {
	code    int
	message string
}

func NewError(code int, message string) *Error {
	return &Error{
		code:    code,
		message: message,
	}
}

func (e *Error) Error() string {
	return fmt.Sprintf("%d  : - %s  ", e.code, e.message)
}

func (e *Error) ErrorCode() int {
	return e.code
}

func (e *Error) String() string {
	return e.Error()
}

func (e *Error) Extend(message string) *Error {
	return &Error{
		code:    e.code,
		message: fmt.Sprintf("%s - %s", e.message, message),
	}
}

func (e *Error) Override(errs ...error) error {
	extraInfo := make([]string, len(errs))
	for i, err := range errs {
		extraInfo[i] = err.Error()
	}
	return fmt.Errorf("%s\nAdditional errors:\n%s", e, strings.Join(extraInfo, "\n"))
}

// IsRetriable returns true if the error is retriable (500-599)
func (e *Error) IsRetriable() bool {
	return e.code >= 500 && e.code < 600
}
