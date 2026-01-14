package cmd

import (
	"fmt"

	"github.com/olimci/shizuka/pkg/events"
)

type eventCounts struct {
	Debug int
	Info  int
	Error int
}

func countEvents(eventsList []events.Event) eventCounts {
	var counts eventCounts
	for _, event := range eventsList {
		switch event.Level {
		case events.Debug:
			counts.Debug++
		case events.Info:
			counts.Info++
		case events.Error:
			counts.Error++
		}
	}
	return counts
}

func levelLabel(level events.Level) string {
	switch level {
	case events.Debug:
		return "debug"
	case events.Info:
		return "info"
	case events.Error:
		return "error"
	default:
		return "event"
	}
}

func formatEvent(event events.Event) string {
	label := levelLabel(event.Level)
	if event.Error != nil {
		return fmt.Sprintf("[%s] %s: %s", label, event.Message, event.Error.Error())
	}
	return fmt.Sprintf("[%s] %s", label, event.Message)
}

func formatSummary(summary *events.Summary) []string {
	if summary == nil || len(summary.Full) == 0 {
		return nil
	}

	counts := countEvents(summary.Full)
	total := len(summary.Full)

	lines := []string{
		fmt.Sprintf(
			"summary: %d events (debug %d, info %d, error %d)",
			total,
			counts.Debug,
			counts.Info,
			counts.Error,
		),
	}

	if summary.ErrorCount > 0 {
		lines = append(lines, fmt.Sprintf("errors (%d):", summary.ErrorCount))
		for _, event := range summary.Errors {
			lines = append(lines, fmt.Sprintf("- %s", formatEvent(event)))
		}
	}

	return lines
}

func hasSummaryEvents(summary *events.Summary) bool {
	return summary != nil && len(summary.Full) > 0
}
