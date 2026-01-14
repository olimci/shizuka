package events

func NewNoopHandler() *NoopHandler {
	return &NoopHandler{}
}

type NoopHandler struct{}

func (h *NoopHandler) Handle(event Event) {}
