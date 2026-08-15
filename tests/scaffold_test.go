package tests

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/scaffold"
)

func scaffolded(t *testing.T, opts scaffold.Options) scaffold.Project {
	t.Helper()
	if opts.Dir == "" {
		opts.Dir = filepath.Join(t.TempDir(), opts.Name)
	}
	project, err := scaffold.Create(opts)
	if err != nil {
		t.Fatalf("creating the project: %v", err)
	}
	return project
}

func read(t *testing.T, project scaffold.Project, name string) string {
	t.Helper()
	raw, err := os.ReadFile(filepath.Join(project.Dir, name))
	if err != nil {
		t.Fatalf("reading %s: %v", name, err)
	}
	return string(raw)
}

func TestScaffoldWritesARunnableProject(t *testing.T) {
	project := scaffolded(t, scaffold.Options{Name: "myshop"})

	for _, name := range []string{
		"main.go",
		"settings.go",
		filepath.Join("migrations", "migrations.go"),
		filepath.Join("templates", "layouts", "base.html"),
		filepath.Join("templates", "pages", "home.html"),
		filepath.Join("static", "site.css"),
		".env",
		".env.example",
		".gitignore",
		"README.md",
	} {
		if _, err := os.Stat(filepath.Join(project.Dir, name)); err != nil {
			t.Errorf("missing %s: %v", name, err)
		}
	}
}

func TestScaffoldedGoFilesParse(t *testing.T) {
	project := scaffolded(t, scaffold.Options{Name: "myshop"})

	for _, name := range []string{"main.go", "settings.go", filepath.Join("migrations", "migrations.go")} {
		path := filepath.Join(project.Dir, name)
		if _, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors); err != nil {
			t.Errorf("%s does not parse: %v", name, err)
		}
	}
}

func TestScaffoldedGoFilesCarryNoComments(t *testing.T) {
	project := scaffolded(t, scaffold.Options{Name: "myshop"})

	for _, name := range []string{"main.go", "settings.go", filepath.Join("migrations", "migrations.go")} {
		path := filepath.Join(project.Dir, name)
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		for _, group := range parsed.Comments {
			for _, comment := range group.List {
				if strings.HasPrefix(comment.Text, "//go:embed") {
					continue
				}
				t.Errorf("%s carries a comment: %s", name, comment.Text)
			}
		}
	}
}

func TestScaffoldWiresTheModulePathThrough(t *testing.T) {
	project := scaffolded(t, scaffold.Options{Name: "myshop", Module: "github.com/jane/myshop"})

	if project.Module != "github.com/jane/myshop" {
		t.Errorf("Module = %q", project.Module)
	}
	if main := read(t, project, "main.go"); !strings.Contains(main, `_ "github.com/jane/myshop/migrations"`) {
		t.Error("main.go should blank import the project's own migrations package")
	}
}

func TestScaffoldDefaultsTheModuleToTheName(t *testing.T) {
	project := scaffolded(t, scaffold.Options{Name: "myshop"})

	if project.Module != "myshop" {
		t.Errorf("Module = %q, want the project name", project.Module)
	}
	if project.Title != "Myshop" {
		t.Errorf("Title = %q", project.Title)
	}
}

func TestScaffoldGeneratesAUsableSecretKey(t *testing.T) {
	first := scaffolded(t, scaffold.Options{Name: "one"})
	second := scaffolded(t, scaffold.Options{Name: "two"})

	if len(first.SecretKey) < 32 {
		t.Errorf("SecretKey is %d characters, too short to be safe", len(first.SecretKey))
	}
	if first.SecretKey == second.SecretKey {
		t.Error("every project must get its own secret key")
	}

	env := read(t, first, ".env")
	if !strings.Contains(env, first.SecretKey) {
		t.Error(".env should carry the generated key")
	}
	if example := read(t, first, ".env.example"); strings.Contains(example, first.SecretKey) {
		t.Error(".env.example must never carry the real key")
	}
	if ignored := read(t, first, ".gitignore"); !strings.Contains(ignored, ".env") {
		t.Error(".env must be git-ignored, it holds the secret key")
	}
}

func TestScaffoldLeavesGoTemplateSyntaxAlone(t *testing.T) {
	project := scaffolded(t, scaffold.Options{Name: "myshop"})

	layout := read(t, project, filepath.Join("templates", "layouts", "base.html"))
	for _, want := range []string{`{{define "base.html"}}`, `{{block "content" .}}`, `{{url "home"}}`} {
		if !strings.Contains(layout, want) {
			t.Errorf("the layout lost %s during scaffolding", want)
		}
	}
	if !strings.Contains(layout, "Myshop") {
		t.Error("the layout should carry the project title")
	}
}

func TestScaffoldRefusesANonEmptyDirectory(t *testing.T) {
	dir := t.TempDir()
	if err := os.WriteFile(filepath.Join(dir, "existing.txt"), []byte("x"), 0o600); err != nil {
		t.Fatal(err)
	}

	if _, err := scaffold.Create(scaffold.Options{Name: "myshop", Dir: dir}); err == nil {
		t.Fatal("scaffolding into a non-empty directory should be refused")
	}

	if _, err := scaffold.Create(scaffold.Options{Name: "myshop", Dir: dir, Force: true}); err != nil {
		t.Errorf("--force should allow it: %v", err)
	}
}

func TestScaffoldRejectsBadNamesAndModules(t *testing.T) {
	for _, name := range []string{"", "my shop", "my/shop", "-shop", "sh op!"} {
		if _, err := scaffold.Create(scaffold.Options{Name: name, Dir: t.TempDir()}); err == nil {
			t.Errorf("name %q should be rejected", name)
		}
	}
	for _, module := range []string{"/leading", "trailing/", "has space", "with/../dots"} {
		if _, err := scaffold.Create(scaffold.Options{Name: "ok", Module: module, Dir: t.TempDir()}); err == nil {
			t.Errorf("module %q should be rejected", module)
		}
	}
}
