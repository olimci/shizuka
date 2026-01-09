package events

type Level uint8

const (
	Debug Level = iota
	Info
	Error
)

func (l Level) String() string {
	switch l {
	case Debug:
		return "D"
	case Info:
		return "I"
	case Error:
		return "E"
	default:
		return "X"
	}
}

type Event struct {
	Level   Level
	Message string
	Error   error
}

type Handler interface {
	Handle(event Event)
}
