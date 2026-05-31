package logging

import (
	"bytes"
	"fmt"
	"log/slog"
	"path/filepath"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"encoding/json"
)

const (
	ansiReset    = "\x1b[0m"
	ansiDim      = "\x1b[2m"
	ansiBlackFg  = "\x1b[30m"
	ansiRed      = "\x1b[31m"
	ansiGreen    = "\x1b[32m"
	ansiYellow   = "\x1b[33m"
	ansiBlue     = "\x1b[34m"
	ansiRedBg    = "\x1b[41m"
	ansiGreenBg  = "\x1b[42m"
	ansiYellowBg = "\x1b[43m"
	ansiBlueBg   = "\x1b[44m"
)

func ansi(code, s string) string { return code + s + ansiReset }

func (h *Handler) format(r slog.Record, color bool) ([]byte, error) {
	var buf bytes.Buffer

	useColor := color && h.opts.Color

	buf.WriteString(levelChip(r.Level, useColor))
	buf.WriteByte(' ')

	if r.Message != "" {
		if useColor {
			buf.WriteString(ansi(levelColor(r.Level), r.Message))
		} else {
			buf.WriteString(r.Message)
		}
	}

	component, step := h.componentStep(r)
	if suffix := formatComponentStep(component, step, useColor); suffix != "" {
		if r.Message != "" {
			buf.WriteByte(' ')
		}
		buf.WriteString(suffix)
	}

	var attrs bytes.Buffer

	if h.opts.AddSource && r.PC != 0 {
		appendTextAttr(&attrs, nil, slog.String("source", source(r.PC)), useColor)
	}

	for _, ga := range h.attrs {
		if len(ga.groups) == 0 && (ga.attr.Key == "component" || ga.attr.Key == "step") {
			continue
		}
		appendTextAttr(&attrs, ga.groups, ga.attr, useColor)
	}

	r.Attrs(func(attr slog.Attr) bool {
		if len(h.groups) == 0 && (attr.Key == "component" || attr.Key == "step") {
			return true
		}
		appendTextAttr(&attrs, h.groups, attr, useColor)
		return true
	})

	if attrs.Len() > 0 {
		if buf.Len() > 0 {
			buf.WriteString("  ")
		}

		buf.Write(attrs.Bytes())
	}

	buf.WriteByte('\n')
	return buf.Bytes(), nil
}

func (h *Handler) formatJSON(r slog.Record) ([]byte, error) {
	root := map[string]any{}

	if !r.Time.IsZero() {
		root["time"] = r.Time.Format(time.RFC3339Nano)
	}

	root["level"] = r.Level.String()
	root["msg"] = r.Message

	if h.opts.AddSource && r.PC != 0 {
		root["source"] = source(r.PC)
	}

	for _, ga := range h.attrs {
		addJSONAttr(root, ga.groups, ga.attr)
	}

	r.Attrs(func(attr slog.Attr) bool {
		addJSONAttr(root, h.groups, attr)
		return true
	})

	line, err := json.Marshal(root)
	if err != nil {
		return nil, err
	}

	line = append(line, '\n')
	return line, nil
}

// componentStep extracts the latest "component" and "step" attrs from the
// handler's accumulated attrs (top-level, no group). Record-level attrs are
// also checked so callers can override per-call.
func (h *Handler) componentStep(r slog.Record) (string, string) {
	var component, step string

	for _, ga := range h.attrs {
		if len(ga.groups) != 0 {
			continue
		}
		switch ga.attr.Key {
		case "component":
			component = ga.attr.Value.Resolve().String()
		case "step":
			step = ga.attr.Value.Resolve().String()
		}
	}

	r.Attrs(func(attr slog.Attr) bool {
		if len(h.groups) != 0 {
			return true
		}
		switch attr.Key {
		case "component":
			component = attr.Value.Resolve().String()
		case "step":
			step = attr.Value.Resolve().String()
		}
		return true
	})

	return component, step
}

func formatComponentStep(component, step string, color bool) string {
	var s string
	switch {
	case component != "" && step != "":
		s = component + "/" + step
	case component != "":
		s = component
	case step != "":
		s = step
	default:
		return ""
	}

	if color {
		return ansi(ansiDim, s)
	}
	return s
}

func appendTextAttr(buf *bytes.Buffer, groups []string, attr slog.Attr, color bool) {
	attr = resolveAttr(attr)
	if attr.Equal(slog.Attr{}) {
		return
	}

	if attr.Key == "" && attr.Value.Kind() != slog.KindGroup {
		return
	}

	if attr.Value.Kind() == slog.KindGroup {
		nextGroups := groups
		if attr.Key != "" {
			nextGroups = append(slices.Clone(groups), attr.Key)
		}

		for _, child := range attr.Value.Group() {
			appendTextAttr(buf, nextGroups, child, color)
		}

		return
	}

	if buf.Len() > 0 {
		buf.WriteByte(' ')
	}

	key := strings.Join(append(slices.Clone(groups), attr.Key), ".")

	if color {
		buf.WriteString(ansi(ansiDim, key+"="))
	} else {
		buf.WriteString(key)
		buf.WriteByte('=')
	}

	buf.WriteString(formatTextValue(attr.Value))
}

func formatTextValue(v slog.Value) string {
	v = v.Resolve()

	switch v.Kind() {
	case slog.KindString:
		return maybeQuote(v.String())
	case slog.KindTime:
		return v.Time().Format(time.RFC3339Nano)
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindBool:
		return strconv.FormatBool(v.Bool())
	case slog.KindInt64:
		return strconv.FormatInt(v.Int64(), 10)
	case slog.KindUint64:
		return strconv.FormatUint(v.Uint64(), 10)
	case slog.KindFloat64:
		return strconv.FormatFloat(v.Float64(), 'g', -1, 64)
	case slog.KindLogValuer:
		return formatTextValue(v.Resolve())
	default:
		return maybeQuote(fmt.Sprint(v.Any()))
	}
}

func addJSONAttr(root map[string]any, groups []string, attr slog.Attr) {
	attr = resolveAttr(attr)
	if attr.Equal(slog.Attr{}) {
		return
	}

	if attr.Key == "" && attr.Value.Kind() != slog.KindGroup {
		return
	}

	if attr.Value.Kind() == slog.KindGroup {
		nextGroups := groups
		if attr.Key != "" {
			nextGroups = append(slices.Clone(groups), attr.Key)
		}

		for _, child := range attr.Value.Group() {
			addJSONAttr(root, nextGroups, child)
		}

		return
	}

	target := root
	for _, group := range groups {
		next, ok := target[group].(map[string]any)
		if !ok {
			next = map[string]any{}
			target[group] = next
		}

		target = next
	}

	target[attr.Key] = jsonValue(attr.Value)
}

func jsonValue(v slog.Value) any {
	v = v.Resolve()

	switch v.Kind() {
	case slog.KindString:
		return v.String()
	case slog.KindBool:
		return v.Bool()
	case slog.KindInt64:
		return v.Int64()
	case slog.KindUint64:
		return v.Uint64()
	case slog.KindFloat64:
		return v.Float64()
	case slog.KindTime:
		return v.Time().Format(time.RFC3339Nano)
	case slog.KindDuration:
		return v.Duration().String()
	case slog.KindGroup:
		m := map[string]any{}
		for _, attr := range v.Group() {
			addJSONAttr(m, nil, attr)
		}
		return m
	case slog.KindLogValuer:
		return jsonValue(v.Resolve())
	case slog.KindAny:
		return v.Any()
	default:
		return fmt.Sprint(v.Any())
	}
}

func resolveAttr(attr slog.Attr) slog.Attr {
	attr.Value = attr.Value.Resolve()

	if attr.Value.Kind() == slog.KindGroup {
		children := attr.Value.Group()
		resolved := make([]slog.Attr, 0, len(children))

		for _, child := range children {
			child = resolveAttr(child)
			if !child.Equal(slog.Attr{}) {
				resolved = append(resolved, child)
			}
		}

		attr.Value = slog.GroupValue(resolved...)
	}

	return attr
}

func source(pc uintptr) string {
	fs := runtime.CallersFrames([]uintptr{pc})
	frame, _ := fs.Next()

	if frame.File == "" {
		return ""
	}

	return filepath.Base(frame.File) + ":" + strconv.Itoa(frame.Line)
}

func levelColor(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return ansiRed
	case level >= slog.LevelWarn:
		return ansiYellow
	case level >= slog.LevelInfo:
		return ansiGreen
	default:
		return ansiBlue
	}
}

func levelLetter(level slog.Level) string {
	switch {
	case level >= slog.LevelError:
		return "E"
	case level >= slog.LevelWarn:
		return "W"
	case level >= slog.LevelInfo:
		return "I"
	default:
		return "D"
	}
}

// levelChip returns a one-character tag for the log level. Colored mode uses
// the level color as a background with black text; plain mode uses a
// bracketed letter so log files stay readable.
func levelChip(level slog.Level, color bool) string {
	letter := levelLetter(level)

	if !color {
		return "[" + letter + "]"
	}

	var bg string
	switch {
	case level >= slog.LevelError:
		bg = ansiRedBg
	case level >= slog.LevelWarn:
		bg = ansiYellowBg
	case level >= slog.LevelInfo:
		bg = ansiGreenBg
	default:
		bg = ansiBlueBg
	}

	return bg + ansiBlackFg + letter + ansiReset
}

func maybeQuote(s string) string {
	if s == "" {
		return `""`
	}

	for _, r := range s {
		if r <= ' ' || r == '"' || r == '=' || r == '\\' {
			return strconv.Quote(s)
		}
	}

	return s
}
