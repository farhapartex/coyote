package tests

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/contrib/scaffold"
)

func TestStartAppScaffoldsAFeaturePackage(t *testing.T) {
	root := t.TempDir()

	feature, err := scaffold.AddFeature(root, "invoices")
	if err != nil {
		t.Fatal(err)
	}

	if feature.Package != "invoices" || feature.Type != "Invoice" {
		t.Errorf("feature = %+v; a plural package should give a singular type", feature)
	}

	for _, name := range []string{"models.go", "admin.go", "handlers.go", "register.go"} {
		path := filepath.Join(root, "internal", "invoices", name)
		if _, err := os.Stat(path); err != nil {
			t.Errorf("missing %s: %v", name, err)
			continue
		}
		if _, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.AllErrors); err != nil {
			t.Errorf("%s does not parse: %v", name, err)
		}
	}

	if _, err := os.Stat(filepath.Join(root, "templates", "pages", "invoices.html")); err != nil {
		t.Errorf("the feature should bring a page template: %v", err)
	}
}

func TestStartAppNamesThingsConsistently(t *testing.T) {
	root := t.TempDir()
	if _, err := scaffold.AddFeature(root, "invoices"); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(filepath.Join(root, "internal", "invoices", "register.go"))
	if err != nil {
		t.Fatal(err)
	}
	source := string(body)
	for _, want := range []string{
		"package invoices",
		"model.Of(Invoice{})",
		"invoiceResource{}",
		`a.Get("/invoices"`,
	} {
		if !strings.Contains(source, want) {
			t.Errorf("register.go should contain %q:\n%s", want, source)
		}
	}
}

func TestStartAppSingularises(t *testing.T) {
	for name, want := range map[string]string{
		"invoices":   "Invoice",
		"categories": "Category",
		"batches":    "Batch",
		"stock":      "Stock",
		"press":      "Press",
	} {
		root := t.TempDir()
		feature, err := scaffold.AddFeature(root, name)
		if err != nil {
			t.Fatalf("%s: %v", name, err)
		}
		if feature.Type != want {
			t.Errorf("%s gave type %q, want %q", name, feature.Type, want)
		}
	}
}

func TestStartAppRefusesToOverwrite(t *testing.T) {
	root := t.TempDir()
	if _, err := scaffold.AddFeature(root, "invoices"); err != nil {
		t.Fatal(err)
	}

	if _, err := scaffold.AddFeature(root, "invoices"); !errors.Is(err, scaffold.ErrFeatureExists) {
		t.Errorf("error = %v, want ErrFeatureExists", err)
	}
}

func TestStartAppRejectsBadNames(t *testing.T) {
	for _, name := range []string{"", "my invoices", "my-invoices", "../escape", "Invoices!"} {
		if _, err := scaffold.AddFeature(t.TempDir(), name); err == nil {
			t.Errorf("name %q should be refused", name)
		}
	}
}

func TestStartAppKeepsAnExistingPage(t *testing.T) {
	root := t.TempDir()
	page := filepath.Join(root, "templates", "pages", "invoices.html")
	if err := os.MkdirAll(filepath.Dir(page), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(page, []byte("mine"), 0o644); err != nil {
		t.Fatal(err)
	}

	if _, err := scaffold.AddFeature(root, "invoices"); err != nil {
		t.Fatal(err)
	}

	body, err := os.ReadFile(page)
	if err != nil {
		t.Fatal(err)
	}
	if string(body) != "mine" {
		t.Error("an existing template must not be overwritten")
	}
}

func TestGeneratedFeatureFilesCarryNoComments(t *testing.T) {
	root := t.TempDir()
	if _, err := scaffold.AddFeature(root, "invoices"); err != nil {
		t.Fatal(err)
	}

	for _, name := range []string{"models.go", "admin.go", "handlers.go", "register.go"} {
		path := filepath.Join(root, "internal", "invoices", name)
		parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ParseComments)
		if err != nil {
			t.Fatal(err)
		}
		for _, group := range parsed.Comments {
			for _, comment := range group.List {
				if strings.HasPrefix(comment.Text, "//go:") {
					continue
				}
				t.Errorf("%s carries a comment: %s", name, comment.Text)
			}
		}
	}
}
