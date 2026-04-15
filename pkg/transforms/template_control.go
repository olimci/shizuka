package transforms

import "errors"

type DiscardError struct{}

func (e *DiscardError) Error() string {
	return "discard artefact"
}

func IsDiscardError(err error) bool {
	var discardErr *DiscardError
	return errors.As(err, &discardErr)
}
