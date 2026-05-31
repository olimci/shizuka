package frontmatter

import "bytes"

func detect(b []byte) (string, int, int, int) {
	if len(b) == 0 {
		return "", 0, 0, 0
	}

	switch {
	case hasPrefixAtLineStart(b, []byte("---")):
		return scanFencedBlock(b, []byte("---"), "yaml")
	case hasPrefixAtLineStart(b, []byte("+++")):
		return scanFencedBlock(b, []byte("+++"), "toml")
	default:
		if kind, start, end, bodyStart := scanJSONObjectPrefix(b); kind != "" {
			return kind, start, end, bodyStart
		}
		return "", 0, 0, 0
	}
}

func scanFencedBlock(b []byte, fence []byte, kind string) (string, int, int, int) {
	openLineEnd := lineEnd(b, 0)
	line := bytes.TrimRight(b[0:openLineEnd], " \t\r\n")
	if !bytes.Equal(line, fence) {
		return "", 0, 0, 0
	}

	payloadStart := openLineEnd
	i := payloadStart

	for i < len(b) {
		nextEnd := lineEnd(b, i)
		rawLine := b[i:nextEnd]
		lineStripped := bytes.TrimRight(rawLine, " \t\r\n")
		if bytes.Equal(lineStripped, fence) {
			payloadEnd := i
			bodyStart := nextEnd
			return kind, payloadStart, payloadEnd, bodyStart
		}
		i = nextEnd
	}
	return "", 0, 0, 0
}

func scanJSONObjectPrefix(b []byte) (string, int, int, int) {
	if len(b) == 0 || b[0] != '{' {
		return "", 0, 0, 0
	}

	var (
		depth          = 0
		inStr          = false
		escaped        = false
		inLineComment  = false
		inBlockComment = false
	)

	for i := 0; i < len(b); i++ {
		c := b[i]

		if inLineComment {
			if c == '\n' {
				inLineComment = false
			}
			continue
		}
		if inBlockComment {
			if c == '*' && i+1 < len(b) && b[i+1] == '/' {
				inBlockComment = false
				i++
			}
			continue
		}
		if inStr {
			if escaped {
				escaped = false
				continue
			}
			switch c {
			case '\\':
				escaped = true
			case '"':
				inStr = false
			}
			continue
		}

		switch c {
		case '"':
			inStr = true
		case '/':
			if i+1 >= len(b) {
				continue
			}
			switch b[i+1] {
			case '/':
				inLineComment = true
				i++
			case '*':
				inBlockComment = true
				i++
			}
		case '{':
			depth++
		case '}':
			depth--
			if depth == 0 {
				end := i + 1
				bodyStart := skipSingleLineEnding(b, end)
				return "json", 0, end, bodyStart
			}
			if depth < 0 {
				return "", 0, 0, 0
			}
		}
	}

	return "", 0, 0, 0
}

func skipSingleLineEnding(b []byte, i int) int {
	if i < len(b) && b[i] == '\r' {
		i++
	}
	if i < len(b) && b[i] == '\n' {
		i++
	}
	return i
}

func hasPrefixAtLineStart(b, prefix []byte) bool {
	if !bytes.HasPrefix(b, prefix) {
		return false
	}
	end := lineEnd(b, 0)
	line := bytes.TrimRight(b[:end], " \t\r\n")
	return bytes.Equal(line, prefix)
}

func lineEnd(b []byte, start int) int {
	i := start
	for i < len(b) && b[i] != '\n' {
		i++
	}
	if i < len(b) && b[i] == '\n' {
		return i + 1
	}
	return i
}

func trimBOM(b []byte) []byte {
	const (
		b0 = 0xEF
		b1 = 0xBB
		b2 = 0xBF
	)
	if len(b) >= 3 && b[0] == b0 && b[1] == b1 && b[2] == b2 {
		return b[3:]
	}
	return b
}
