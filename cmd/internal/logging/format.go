package logging

import "fmt"

const formatUsage = "auto, plain, pretty, or json"

func ParseFormat(value string) (Format, error) {
	switch value {
	case "", "auto":
		return FormatAuto, nil
	case "plain":
		return FormatPlain, nil
	case "pretty":
		return FormatPretty, nil
	case "json":
		return FormatJSON, nil
	default:
		return FormatUnset, fmt.Errorf("invalid output format %q: expected %s", value, formatUsage)
	}
}

func ValidateFormat(value string) error {
	_, err := ParseFormat(value)
	return err
}
