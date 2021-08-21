package types

import "errors"

var (
	ErrAPIRouteNotFound  = errors.New("API Route not found.")
	ErrAlreadyAccepted   = errors.New("This Sprout ID was already accepted.")
	ErrAlreadyDenied     = errors.New("This Sprout ID was already denied.")
	ErrAlreadyRejected   = errors.New("This Sprout ID was already rejected.")
	ErrAlreadyUnaccepted = errors.New("This Sprout ID was already unaccepted.")
	ErrCannotParseRootCA = errors.New("Cannot load the RootCA certificate.")
	ErrSproutIDFound     = errors.New("A Sprout ID matching that system has already been recorded.")
	ErrSproutIDNotFound  = errors.New("A Sprout ID matching that system cannot be found.")
)
