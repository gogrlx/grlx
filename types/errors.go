package types

import "errors"

var (
	ErrAPIRouteNotFound     = errors.New("API Route not found")
	ErrAlreadyAccepted      = errors.New("this Sprout ID was already accepted")
	ErrAlreadyDenied        = errors.New("this Sprout ID was already denied")
	ErrAlreadyRejected      = errors.New("this Sprout ID was already rejected")
	ErrAlreadyUnaccepted    = errors.New("this Sprout ID was already unaccepted")
	ErrCannotParseRootCA    = errors.New("cannot load the RootCA certificate")
	ErrDependencyCycleFound = errors.New("found a dependency cycle")
	ErrSproutIDFound        = errors.New("a Sprout ID matching that system has already been recorded")
	ErrSproutIDInvalid      = errors.New("bad user input: invalid SproutID received")
	ErrSproutIDNotFound     = errors.New("a Sprout ID matching that system cannot be found")
	ErrInvalidUserInput     = errors.New("invalid user input was received")

	ErrNotImplemented           = errors.New("this feature is not yet implemented")
	ErrInvalidKeyState          = errors.New("code bug: an invalid key state was supplied")
	ErrConfirmationLengthIsZero = errors.New("code bug: confirmation options muct not be 0-length")

	ErrInvalidMethod  = errors.New("invalid method")
	ErrMissingName    = errors.New("recipe is missing a name")
	ErrMissingSource  = errors.New("recipe is missing a source")
	ErrMissingHash    = errors.New("file is missing a hash")
	ErrCacheFailure   = errors.New("file caching failed")
	ErrMissingContent = errors.New("file is missing content")

	ErrFileNotFound  = errors.New("file not found")
	ErrHashMismatch  = errors.New("file hash mismatch")
	ErrDeleteRoot    = errors.New("cannot delete root directory")
	ErrModifyRoot    = errors.New("cannot modify root directory")
	ErrMissingTarget = errors.New("target is missing")
	ErrPathNotFound  = errors.New("path not found")
)
