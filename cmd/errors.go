package cmd

import "errors"

type silentError struct {
	err error
}

func (e *silentError) Error() string {
	if e == nil || e.err == nil {
		return ""
	}
	return e.err.Error()
}

func (e *silentError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.err
}

func quietError(err error) error {
	if err == nil {
		return nil
	}
	return &silentError{err: err}
}

func IsSilentError(err error) bool {
	var target *silentError
	return errors.As(err, &target)
}
