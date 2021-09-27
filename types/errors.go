package types

import "errors"

var (
	ErrAPIRouteNotFound     = errors.New("API Route not found.")
	ErrAlreadyAccepted      = errors.New("This Sprout ID was already accepted.")
	ErrAlreadyDenied        = errors.New("This Sprout ID was already denied.")
	ErrAlreadyRejected      = errors.New("This Sprout ID was already rejected.")
	ErrAlreadyUnaccepted    = errors.New("This Sprout ID was already unaccepted.")
	ErrCannotParseRootCA    = errors.New("Cannot load the RootCA certificate.")
	ErrDependencyCycleFound = errors.New("Found a dependency cycle!")
	ErrSproutIDFound        = errors.New("A Sprout ID matching that system has already been recorded.")
	ErrSproutIDInvalid      = errors.New("Bad user input: invalid SproutID received")
	ErrSproutIDNotFound     = errors.New("A Sprout ID matching that system cannot be found.")
	ErrInvalidUserInput     = errors.New("Invalid user input was received")

	ErrInvalidKeyState          = errors.New("Code bug: an invalid key state was supplied")
	ErrConfirmationLengthIsZero = errors.New("Code bug: confirmation options muct not be 0-length")
)
