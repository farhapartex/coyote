package scaffold

import (
	"bytes"
	"crypto/rand"
	"embed"
	"encoding/base64"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed files
var files embed.FS

var (
	ErrNotEmpty      = errors.New("coyote/scaffold: the target directory is not empty")
	ErrInvalidName   = errors.New("coyote/scaffold: invalid project name")
	ErrInvalidModule = errors.New("coyote/scaffold: invalid module path")
)

type Options struct {
	Dir    string
	Name   string
	Module string
	Force  bool
}

type Project struct {
	Dir       string
	Name      string
	Module    string
	Title     string
	SecretKey string
	Files     []string
}

type source struct {
	template string
	path     string
}

var layout = []source{
	{"main.go.tmpl", "main.go"},
	{"settings.go.tmpl", "settings.go"},
	{"migrations.go.tmpl", filepath.Join("migrations", "migrations.go")},
	{"base.html.tmpl", filepath.Join("templates", "layouts", "base.html")},
	{"home.html.tmpl", filepath.Join("templates", "pages", "home.html")},
	{"site.css.tmpl", filepath.Join("static", "site.css")},
	{"env.tmpl", ".env"},
	{"env.example.tmpl", ".env.example"},
	{"gitignore.tmpl", ".gitignore"},
	{"README.md.tmpl", "README.md"},
}

func Create(opts Options) (Project, error) {
	project, err := plan(opts)
	if err != nil {
		return Project{}, err
	}
	if err := ensureEmpty(project.Dir, opts.Force); err != nil {
		return Project{}, err
	}
	for _, item := range layout {
		if err := write(project, item); err != nil {
			return Project{}, err
		}
		project.Files = append(project.Files, item.path)
	}
	return project, nil
}

func plan(opts Options) (Project, error) {
	name := strings.TrimSpace(opts.Name)
	if !ValidName(name) {
		return Project{}, fmt.Errorf("%w: %q; use letters, digits, hyphens and underscores", ErrInvalidName, opts.Name)
	}

	module := strings.TrimSpace(opts.Module)
	if module == "" {
		module = name
	}
	if !ValidModule(module) {
		return Project{}, fmt.Errorf("%w: %q", ErrInvalidModule, module)
	}

	dir := opts.Dir
	if dir == "" {
		dir = name
	}
	absolute, err := filepath.Abs(dir)
	if err != nil {
		return Project{}, err
	}

	key, err := SecretKey()
	if err != nil {
		return Project{}, err
	}

	return Project{
		Dir:       absolute,
		Name:      name,
		Module:    module,
		Title:     Titleise(name),
		SecretKey: key,
	}, nil
}

func write(project Project, item source) error {
	raw, err := files.ReadFile("files/" + item.template)
	if err != nil {
		return err
	}

	parsed, err := template.New(item.template).Delims("[[", "]]").Parse(string(raw))
	if err != nil {
		return err
	}

	rendered := &bytes.Buffer{}
	if err := parsed.Execute(rendered, project); err != nil {
		return err
	}

	target := filepath.Join(project.Dir, item.path)
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	return os.WriteFile(target, rendered.Bytes(), 0o644)
}

func ensureEmpty(dir string, force bool) error {
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return os.MkdirAll(dir, 0o755)
	}
	if err != nil {
		return err
	}
	if len(entries) > 0 && !force {
		return fmt.Errorf("%w: %s", ErrNotEmpty, dir)
	}
	return nil
}

func SecretKey() (string, error) {
	raw := make([]byte, 36)
	if _, err := rand.Read(raw); err != nil {
		return "", fmt.Errorf("coyote/scaffold: generating a secret key: %w", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw), nil
}
