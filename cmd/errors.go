package cmd

const defaultFailureExitCode = 1

type HandledError struct {
	Code int
	Err  error
}

func handled(err error) error {
	return handledWithCode(defaultFailureExitCode, err)
}

func handledWithCode(code int, err error) error {
	if err == nil {
		return nil
	}
	if code == 0 {
		code = defaultFailureExitCode
	}
	return &HandledError{Code: code, Err: err}
}

func (e *HandledError) Error() string {
	if e == nil || e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *HandledError) Unwrap() error {
	if e == nil {
		return nil
	}
	return e.Err
}
