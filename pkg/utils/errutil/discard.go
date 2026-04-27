package errutil

import "errors"

type DiscardError struct {
	Cause error
}

func (e *DiscardError) Error() string {
	if e == nil || e.Cause == nil {
		return "discard artefact"
	}
	return e.Cause.Error()
}

func (e *DiscardError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Cause
}

func Discard() *DiscardError {
	return &DiscardError{}
}

func WrapDiscard(err error) *DiscardError {
	if err == nil {
		return Discard()
	}

	if discardErr, ok := errors.AsType[*DiscardError](err); ok {
		return discardErr
	}

	return &DiscardError{Cause: err}
}

func IsDiscard(err error) bool {
	var discardErr *DiscardError
	return errors.As(err, &discardErr)
}
