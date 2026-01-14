package events

import "slices"

func NewCollector(handler Handler) *Collector {
	return &Collector{
		Events:  make([]Event, 0),
		handler: handler,
	}
}

type Collector struct {
	Events  []Event
	handler Handler
}

func (c *Collector) Handle(event Event) {
	c.Events = append(c.Events, event)
	c.handler.Handle(event)
}

func (c *Collector) AtLevel(level Level) []Event {
	out := make([]Event, 0)
	for _, event := range c.Events {
		if event.Level >= level {
			out = append(out, event)
		}
	}
	return out
}

func (c *Collector) HasLevel(level Level) bool {
	for _, event := range c.Events {
		if event.Level == level {
			return true
		}
	}

	return false
}

func (c *Collector) MaxLevel() Level {
	max := Level(0)
	for _, event := range c.Events {
		if event.Level > max {
			max = event.Level
		}
	}
	return max
}

func (c *Collector) Clear() {
	c.Events = make([]Event, 0)
}

func (c *Collector) Summary() *Summary {
	out := new(Summary)

	for _, event := range c.Events {
		if event.Level == Error {
			out.ErrorCount++
			out.Errors = append(out.Errors, event)
		}
	}

	out.Full = slices.Clone(c.Events)

	return out
}
