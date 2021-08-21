package types

import "errors"

var (
	ErrSproutIDFound     = errors.New("A Sprout ID matching that system has already been recorded.")
	ErrSproutIDNotFound  = errors.New("A Sprout ID matching that system cannot be found.")
	ErrAlreadyAccepted   = errors.New("This Sprout ID was already accepted.")
	ErrAlreadyDenied     = errors.New("This Sprout ID was already denied.")
	ErrAlreadyUnaccepted = errors.New("This Sprout ID was already unaccepted.")
	ErrAlreadyRejected   = errors.New("This Sprout ID was already rejected.")
)
