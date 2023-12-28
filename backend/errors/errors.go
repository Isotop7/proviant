package errors

import "errors"

var (
	ErrMismatcherUserID = errors.New("mismatching user id of requested product")
)
