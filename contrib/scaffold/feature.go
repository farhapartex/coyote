package scaffold

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"unicode"
)

var ErrFeatureExists = errors.New("coyote/scaffold: that package already exists")

type Feature struct {
	Dir     string
	Package string
	Type    string
	Lower   string
	Title   string
	Files   []string
}

var featureLayout = []source{
	{"app/models.go.tmpl", "models.go"},
	{"app/admin.go.tmpl", "admin.go"},
	{"app/handlers.go.tmpl", "handlers.go"},
	{"app/register.go.tmpl", "register.go"},
}

func AddFeature(root, name string) (Feature, error) {
	name = strings.ToLower(strings.TrimSpace(name))
	if !ValidName(name) || strings.ContainsAny(name, "-") {
		return Feature{}, fmt.Errorf("%w: %q; use letters, digits and underscores", ErrInvalidName, name)
	}

	feature := Feature{
		Dir:     filepath.Join(root, "internal", name),
		Package: name,
		Type:    exported(singular(name)),
		Lower:   singular(name),
		Title:   Titleise(name),
	}

	if _, err := os.Stat(feature.Dir); err == nil {
		return Feature{}, fmt.Errorf("%w: %s", ErrFeatureExists, feature.Dir)
	}
	if err := os.MkdirAll(feature.Dir, 0o755); err != nil {
		return Feature{}, err
	}

	for _, item := range featureLayout {
		if err := writeFile(files, item.template, filepath.Join(feature.Dir, item.path), feature); err != nil {
			return Feature{}, err
		}
		feature.Files = append(feature.Files, filepath.Join("internal", name, item.path))
	}

	page := filepath.Join(root, "templates", "pages", name+".html")
	if _, err := os.Stat(page); errors.Is(err, os.ErrNotExist) {
		if err := os.MkdirAll(filepath.Dir(page), 0o755); err != nil {
			return Feature{}, err
		}
		if err := writeFile(files, "app/page.html.tmpl", page, feature); err != nil {
			return Feature{}, err
		}
		feature.Files = append(feature.Files, filepath.Join("templates", "pages", name+".html"))
	}
	return feature, nil
}

func exported(name string) string {
	if name == "" {
		return name
	}
	runes := []rune(name)
	runes[0] = unicode.ToUpper(runes[0])
	return string(runes)
}

func singular(name string) string {
	switch {
	case strings.HasSuffix(name, "ies"):
		return strings.TrimSuffix(name, "ies") + "y"
	case strings.HasSuffix(name, "sses"), strings.HasSuffix(name, "shes"), strings.HasSuffix(name, "ches"):
		return strings.TrimSuffix(name, "es")
	case strings.HasSuffix(name, "s") && !strings.HasSuffix(name, "ss"):
		return strings.TrimSuffix(name, "s")
	}
	return name
}
