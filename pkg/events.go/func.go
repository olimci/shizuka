package events

func NewHandlerFunc(handle func(event Event)) HandlerFunc {
	return HandlerFunc{
		handle: handle,
	}
}

type HandlerFunc struct {
	handle func(event Event)
}

func (h HandlerFunc) Handle(event Event) {
	h.handle(event)
}
