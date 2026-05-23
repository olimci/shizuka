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

func (h *Handler) format(r slog.Record, color bool) ([]byte, error) {
	var buf bytes.Buffer

	useColor := color && h.opts.Color

	if r.Message != "" {
		buf.WriteString(formatMessage(r.Level, r.Message, useColor))
	}

	var attrs bytes.Buffer

	if h.opts.AddSource && r.PC != 0 {
		appendTextAttr(&attrs, nil, slog.String("source", source(r.PC)), useColor)
	}

	for _, ga := range h.attrs {
		appendTextAttr(&attrs, ga.groups, ga.attr, useColor)
	}

	r.Attrs(func(attr slog.Attr) bool {
		appendTextAttr(&attrs, h.groups, attr, useColor)
		return true
	})

	if attrs.Len() > 0 {
		if buf.Len() > 0 {
			buf.WriteString(" ")
		}

		if useColor {
			buf.WriteString(colorDim("["))
			buf.Write(attrs.Bytes())
			buf.WriteString(colorDim("]"))
		} else {
			buf.WriteString("[")
			buf.Write(attrs.Bytes())
			buf.WriteString("]")
		}
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

func appendTextAttr(buf *bytes.Buffer, groups []string, attr slog.Attr, color bool) {
	attr = resolveAttr(attr)
	if attr.Equal(slog.Attr{}) {
		return
	}

	appendTextAttrValue(buf, groups, attr, color)
}

func appendTextAttrValue(buf *bytes.Buffer, groups []string, attr slog.Attr, color bool) {
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
		buf.WriteString(colorKey(key))
		buf.WriteString(colorDim("="))
	} else {
		buf.WriteString(key)
		buf.WriteByte('=')
	}

	buf.WriteString(formatTextValue(attr.Value, color))
}

func formatTextValue(v slog.Value, color bool) string {
	v = v.Resolve()

	var s string

	switch v.Kind() {
	case slog.KindString:
		s = strconv.Quote(v.String())
	case slog.KindTime:
		s = strconv.Quote(v.Time().Format(time.RFC3339Nano))
	case slog.KindDuration:
		s = strconv.Quote(v.Duration().String())
	case slog.KindBool:
		s = strconv.FormatBool(v.Bool())
	case slog.KindInt64:
		s = strconv.FormatInt(v.Int64(), 10)
	case slog.KindUint64:
		s = strconv.FormatUint(v.Uint64(), 10)
	case slog.KindFloat64:
		s = strconv.FormatFloat(v.Float64(), 'g', -1, 64)
	case slog.KindAny:
		s = strconv.Quote(fmt.Sprint(v.Any()))
	case slog.KindLogValuer:
		return formatTextValue(v.Resolve(), color)
	default:
		s = strconv.Quote(fmt.Sprint(v.Any()))
	}

	if color {
		return colorValue(s)
	}

	return s
}

func addJSONAttr(root map[string]any, groups []string, attr slog.Attr) {
	attr = resolveAttr(attr)
	if attr.Equal(slog.Attr{}) {
		return
	}

	addJSONAttrValue(root, groups, attr)
}

func addJSONAttrValue(root map[string]any, groups []string, attr slog.Attr) {
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

func formatMessage(level slog.Level, s string, color bool) string {
	if !color {
		return s
	}

	return colorByLevel(level, s)
}

func colorByLevel(level slog.Level, s string) string {
	const (
		reset  = "\033[0m"
		red    = "\033[31m"
		yellow = "\033[33m"
		blue   = "\033[34m"
		gray   = "\033[90m"
	)

	switch {
	case level >= slog.LevelError:
		return red + s + reset
	case level >= slog.LevelWarn:
		return yellow + s + reset
	case level >= slog.LevelInfo:
		return blue + s + reset
	default:
		return gray + s + reset
	}
}

func colorKey(s string) string {
	const (
		reset = "\033[0m"
		cyan  = "\033[36m"
	)

	return cyan + s + reset
}

func colorValue(s string) string {
	const (
		reset = "\033[0m"
		gray  = "\033[90m"
	)

	return gray + s + reset
}

func colorDim(s string) string {
	const (
		reset = "\033[0m"
		dim   = "\033[2m"
	)

	return dim + s + reset
}
