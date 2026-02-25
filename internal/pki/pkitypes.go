package pki

import "errors"

type (
	KeySubmission struct {
		NKey     string `json:"nkey"`
		SproutID string `json:"id"`
	}
	KeyManager struct {
		SproutID string `json:"id"`
	}
	KeySet struct {
		Sprouts []KeyManager `json:"sprouts"`
	}
	KeysByType struct {
		Accepted   KeySet `json:"accepted,omitempty"`
		Denied     KeySet `json:"denied,omitempty"`
		Rejected   KeySet `json:"rejected,omitempty"`
		Unaccepted KeySet `json:"unaccepted,omitempty"`
	}
)

var (
	ErrAlreadyAccepted          = errors.New("this Sprout ID was already accepted")
	ErrAlreadyDenied            = errors.New("this Sprout ID was already denied")
	ErrAlreadyRejected          = errors.New("this Sprout ID was already rejected")
	ErrAlreadyUnaccepted        = errors.New("this Sprout ID was already unaccepted")
	ErrCannotParseRootCA        = errors.New("cannot load the RootCA certificate")
	ErrSproutIDFound            = errors.New("a Sprout ID matching that system has already been recorded")
	ErrSproutIDInvalid          = errors.New("bad user input: invalid SproutID received")
	ErrSproutIDNotFound         = errors.New("a Sprout ID matching that system cannot be found")
	ErrInvalidKeyState          = errors.New("code bug: an invalid key state was supplied")
	ErrConfirmationLengthIsZero = errors.New("code bug: confirmation options muct not be 0-length")
)
