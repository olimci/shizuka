package scaffold

func defaultOptions() *options {
	return &options{
		variables: make(map[string]any),
		force:     false,
	}
}

type options struct {
	variables map[string]any
	force     bool
}

func (o *options) apply(opts ...Option) *options {
	for _, opt := range opts {
		opt(o)
	}

	return o
}

type Option func(o *options)

func WithVariables(vars map[string]any) Option {
	if vars == nil {
		vars = make(map[string]any)
	}

	return func(o *options) {
		o.variables = vars
	}
}

func WithForce(force bool) Option {
	return func(o *options) {
		o.force = force
	}
}
