package approval

import "errors"

var (
	// ErrSelfApproval is returned when an agent tries to approve its own request.
	ErrSelfApproval = errors.New("cannot approve own request")

	// ErrAlreadyResolved is returned when an approval has already been resolved.
	ErrAlreadyResolved = errors.New("approval already resolved")
)
