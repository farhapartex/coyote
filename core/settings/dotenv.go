package settings

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"regexp"
	"strings"
)

var keyPattern = regexp.MustCompile(`^[A-Za-z_][A-Za-z0-9_.]*$`)

type File struct {
	Path     string
	Optional bool
}

func DotEnv(path string) File { return File{Path: path, Optional: true} }

func RequiredDotEnv(path string) File { return File{Path: path} }

func (f File) Name() string { return f.Path }

func (f File) Values() (map[string]string, error) {
	handle, err := os.Open(f.Path)
	if err != nil {
		if f.Optional && errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("coyote/settings: reading %s: %w", f.Path, err)
	}
	defer handle.Close()

	values, err := ParseDotEnv(handle)
	if err != nil {
		return nil, fmt.Errorf("coyote/settings: %s: %w", f.Path, err)
	}
	return values, nil
}

func LoadDotEnv(paths ...string) error {
	if len(paths) == 0 {
		paths = []string{".env"}
	}
	sources := make([]Source, 0, len(paths))
	for _, path := range paths {
		sources = append(sources, DotEnv(path))
	}
	return Load(sources...)
}

func MustLoadDotEnv(paths ...string) {
	if err := LoadDotEnv(paths...); err != nil {
		panic(err)
	}
}

func ParseDotEnv(reader io.Reader) (map[string]string, error) {
	values := map[string]string{}
	scanner := bufio.NewScanner(reader)
	line := 0

	for scanner.Scan() {
		line++
		text := strings.TrimSpace(scanner.Text())
		if text == "" || strings.HasPrefix(text, "#") {
			continue
		}
		text = strings.TrimPrefix(text, "export ")

		key, raw, found := strings.Cut(text, "=")
		if !found {
			return nil, fmt.Errorf("line %d: expected KEY=value", line)
		}
		key = strings.TrimSpace(key)
		if !keyPattern.MatchString(key) {
			return nil, fmt.Errorf("line %d: %q is not a valid variable name", line, key)
		}
		value, err := unquote(strings.TrimSpace(raw))
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", line, err)
		}
		values[key] = value
	}
	if err := scanner.Err(); err != nil {
		return nil, err
	}
	return values, nil
}

func unquote(raw string) (string, error) {
	if len(raw) >= 2 {
		quote := raw[0]
		if (quote == '"' || quote == '\'') && raw[len(raw)-1] == quote {
			inner := raw[1 : len(raw)-1]
			if quote == '\'' {
				return inner, nil
			}
			return expandEscapes(inner), nil
		}
		if quote == '"' || quote == '\'' {
			return "", errors.New("unterminated quote")
		}
	}
	if index := strings.Index(raw, " #"); index >= 0 {
		raw = raw[:index]
	}
	return strings.TrimSpace(raw), nil
}

func expandEscapes(value string) string {
	replacer := strings.NewReplacer(
		`\n`, "\n",
		`\r`, "\r",
		`\t`, "\t",
		`\"`, `"`,
		`\\`, `\`,
	)
	return replacer.Replace(value)
}
