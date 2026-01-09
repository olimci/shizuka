package events

import (
	"fmt"
	"strings"
)

type Summary struct {
	ErrorCount int

	Errors []Event

	Full []Event
}

func (s Summary) String() string {
	lines := make([]string, len(s.Errors))
	for i, err := range s.Errors {
		if err.Error != nil {
			lines[i] = fmt.Sprintf("- %s (%s)", err.Message, err.Error.Error())
		} else {
			lines[i] = fmt.Sprintf("- %s", err.Message)
		}
	}

	return fmt.Sprintf("Errors (%d):\n%s", s.ErrorCount, strings.Join(lines, "\n"))
}
