package i18n

import (
	"bufio"
	"fmt"
	"io"
	"strconv"
	"strings"
)

type target int

const (
	targetNone target = iota
	targetContext
	targetSingular
	targetPlural
	targetForm
)

type poReader struct {
	current  message
	forms    map[int]string
	into     target
	index    int
	started  bool
	obsolete bool
}

func ParsePO(tag string, source io.Reader) (Catalog, error) {
	catalog := newCatalogue(tag)
	state := &poReader{forms: map[int]string{}}

	scanner := bufio.NewScanner(source)
	scanner.Buffer(make([]byte, 0, 64*1024), 4*1024*1024)

	line := 0
	for scanner.Scan() {
		line++
		text := strings.TrimSpace(strings.TrimPrefix(scanner.Text(), "\ufeff"))

		if text == "" {
			state.flush(catalog)
			continue
		}
		if err := state.read(text); err != nil {
			return nil, fmt.Errorf("%w: line %d: %w", ErrMalformedPO, line, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	state.flush(catalog)
	return catalog, nil
}

func (s *poReader) read(text string) error {
	switch {
	case strings.HasPrefix(text, "#~"):
		s.obsolete = true
		return nil
	case strings.HasPrefix(text, "#,"):
		if strings.Contains(text, "fuzzy") {
			s.current.fuzzy = true
		}
		return nil
	case strings.HasPrefix(text, "#"):
		return nil
	case strings.HasPrefix(text, `"`):
		value, err := unquotePO(text)
		if err != nil {
			return err
		}
		return s.extend(value)
	}

	keyword, rest, found := strings.Cut(text, " ")
	if !found {
		return fmt.Errorf("expected a keyword and a string, got %q", text)
	}
	value, err := unquotePO(strings.TrimSpace(rest))
	if err != nil {
		return err
	}
	return s.begin(keyword, value)
}

func (s *poReader) begin(keyword, value string) error {
	switch keyword {
	case "msgctxt":
		s.started, s.into = true, targetContext
		s.current.context = value
	case "msgid":
		s.started, s.into = true, targetSingular
		s.current.singular = value
	case "msgid_plural":
		s.into = targetPlural
		s.current.plural = value
	case "msgstr":
		s.started, s.into, s.index = true, targetForm, 0
		s.forms[0] = value
	default:
		index, err := pluralIndex(keyword)
		if err != nil {
			return err
		}
		s.into, s.index = targetForm, index
		s.forms[index] = value
	}
	return nil
}

func (s *poReader) extend(value string) error {
	switch s.into {
	case targetContext:
		s.current.context += value
	case targetSingular:
		s.current.singular += value
	case targetPlural:
		s.current.plural += value
	case targetForm:
		s.forms[s.index] += value
	default:
		return fmt.Errorf("a quoted string with no msgid, msgstr or msgctxt before it")
	}
	return nil
}

func (s *poReader) flush(catalog *catalogue) {
	defer s.reset()

	if !s.started || s.obsolete {
		return
	}

	highest := -1
	for index := range s.forms {
		if index > highest {
			highest = index
		}
	}
	s.current.forms = make([]string, highest+1)
	for index, value := range s.forms {
		s.current.forms[index] = value
	}
	catalog.add(s.current)
}

func (s *poReader) reset() {
	s.current = message{}
	s.forms = map[int]string{}
	s.into = targetNone
	s.index = 0
	s.started = false
	s.obsolete = false
}

func pluralIndex(keyword string) (int, error) {
	if !strings.HasPrefix(keyword, "msgstr[") || !strings.HasSuffix(keyword, "]") {
		return 0, fmt.Errorf("unknown keyword %q", keyword)
	}
	index, err := strconv.Atoi(keyword[len("msgstr[") : len(keyword)-1])
	if err != nil || index < 0 || index > 15 {
		return 0, fmt.Errorf("unusable plural index in %q", keyword)
	}
	return index, nil
}

func unquotePO(text string) (string, error) {
	trimmed := strings.TrimSpace(text)
	if len(trimmed) < 2 || !strings.HasPrefix(trimmed, `"`) || !strings.HasSuffix(trimmed, `"`) {
		return "", fmt.Errorf("expected a quoted string, got %q", text)
	}
	return expandPO(trimmed[1 : len(trimmed)-1]), nil
}

func expandPO(value string) string {
	if !strings.ContainsRune(value, '\\') {
		return value
	}

	var b strings.Builder
	for at := 0; at < len(value); at++ {
		if value[at] != '\\' || at+1 >= len(value) {
			b.WriteByte(value[at])
			continue
		}
		at++
		switch value[at] {
		case 'n':
			b.WriteByte('\n')
		case 't':
			b.WriteByte('\t')
		case 'r':
			b.WriteByte('\r')
		case 'a':
			b.WriteByte('\a')
		case 'b':
			b.WriteByte('\b')
		case 'f':
			b.WriteByte('\f')
		case 'v':
			b.WriteByte('\v')
		case '"', '\\', '\'':
			b.WriteByte(value[at])
		default:
			b.WriteByte('\\')
			b.WriteByte(value[at])
		}
	}
	return b.String()
}
