// errors contains custom error definitions
package errors

import "errors"

var (
	// ErrMismatcherUserID occurs if a given user id mismatches the user id of a product owner
	ErrMismatcherUserID = errors.New("mismatching user id of requested product")

	// ErrUserHasNoMailAddress is thrown if a given user has no mail address
	ErrUserHasNoMailAddress = errors.New("user has no mail address")

	// ErrNoBarcodeFoundInImage is thrown when no barcode can be read from an image
	ErrNoBarcodeFoundInImage = errors.New("error reading barcode from image")

	// ErrBarcodeDecodeTimeoutExceeded is thrown when decoding a barcode from an image takes longer than the allowed timeout
	ErrBarcodeDecodeTimeoutExceeded = errors.New("barcode decoding timeout reached")
)
