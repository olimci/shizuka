package cmd

type HandledError struct {
	Code int
	Err  error
}

func (e *HandledError) Error() string { return e.Err.Error() }
func (e *HandledError) Unwrap() error { return e.Err }

func handled(err error) error {
	return &HandledError{Code: 1, Err: err}
}
