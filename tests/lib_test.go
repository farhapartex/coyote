package tests

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/lib/id"
	"github.com/farhapartex/coyote/lib/text"
)

const modulePath = "github.com/farhapartex/coyote"

func TestHumanise(t *testing.T) {
	for input, want := range map[string]string{
		"id":           "ID",
		"sku":          "SKU",
		"api_key":      "API key",
		"first_name":   "First name",
		"is_published": "Is published",
		"users":        "Users",
		"":             "",
		"json_payload": "JSON payload",
	} {
		if got := text.Humanise(input); got != want {
			t.Errorf("Humanise(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestTitleise(t *testing.T) {
	for input, want := range map[string]string{
		"Product":     "Product",
		"OrderLine":   "Order Line",
		"HTTPRequest": "H T T P Request",
		"":            "",
	} {
		if got := text.Titleise(input); got != want {
			t.Errorf("Titleise(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestSlugifyAndHyphenate(t *testing.T) {
	for input, want := range map[string]string{
		"create table users": "create_table_users",
		"  Add Product SKU ": "add_product_sku",
		"already_ok":         "already_ok",
		"!!!":                "",
	} {
		if got := text.Slugify(input); got != want {
			t.Errorf("Slugify(%q) = %q, want %q", input, got, want)
		}
	}
	if got := text.Hyphenate("Server Info"); got != "server-info" {
		t.Errorf("Hyphenate = %q", got)
	}
}

func TestFoldAndBlank(t *testing.T) {
	if got := text.Fold("  MiXeD Case  "); got != "mixed case" {
		t.Errorf("Fold = %q", got)
	}
	if !text.Blank("   ") || text.Blank(" x ") {
		t.Error("Blank should only report whitespace-only values")
	}
}

func TestTruncate(t *testing.T) {
	if got := text.Truncate("short", 10); got != "short" {
		t.Errorf("Truncate = %q", got)
	}
	if got := text.Truncate("abcdefghij", 4); got != "abcd…" {
		t.Errorf("Truncate = %q", got)
	}
	if got := text.Truncate("café society", 4); got != "café…" {
		t.Errorf("Truncate should count runes, got %q", got)
	}
	if got := text.Truncate("anything", 0); got != "anything" {
		t.Errorf("a zero limit should not truncate, got %q", got)
	}
}

func TestIDIsUniqueUUID(t *testing.T) {
	first, err := id.New()
	if err != nil {
		t.Fatal(err)
	}
	second, _ := id.New()

	if len(first) != 36 || strings.Count(first, "-") != 4 {
		t.Errorf("not a uuid: %q", first)
	}
	if first[14] != '4' {
		t.Errorf("expected version 4, got %q", first)
	}
	if first == second {
		t.Error("ids should be unique")
	}
	if id.MustNew() == "" {
		t.Error("MustNew returned empty")
	}
}

func TestLibDependsOnNothingLocal(t *testing.T) {
	forbidden := []string{
		modulePath + "/core",
		modulePath + "/contrib",
		modulePath + "/admin",
		modulePath + "/cmd",
	}
	for _, file := range goFilesUnder(t, "../lib") {
		for _, imported := range importsOf(t, file) {
			for _, prefix := range forbidden {
				if strings.HasPrefix(imported, prefix) {
					t.Errorf("%s imports %s; lib must depend on nothing local", file, imported)
				}
			}
		}
	}
}

var knownInversions = map[string]string{
	"../core/app/serve.go": modulePath + "/contrib/cli",
}

func TestCoreDependsOnNeitherContribNorAdmin(t *testing.T) {
	forbidden := []string{modulePath + "/contrib", modulePath + "/admin", modulePath + "/cmd"}
	seen := map[string]bool{}

	for _, file := range goFilesUnder(t, "../core") {
		for _, imported := range importsOf(t, file) {
			for _, prefix := range forbidden {
				if !strings.HasPrefix(imported, prefix) {
					continue
				}
				if knownInversions[file] == imported {
					seen[file] = true
					continue
				}
				t.Errorf("%s imports %s; core must not depend on contrib, admin or cmd", file, imported)
			}
		}
	}

	for file, imported := range knownInversions {
		if !seen[file] {
			t.Errorf("%s no longer imports %s; remove it from knownInversions", file, imported)
		}
	}
}

func goFilesUnder(t *testing.T, root string) []string {
	t.Helper()
	var found []string
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if !entry.IsDir() && strings.HasSuffix(path, ".go") {
			found = append(found, path)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(found) == 0 {
		t.Fatalf("no Go files under %s", root)
	}
	return found
}

func importsOf(t *testing.T, path string) []string {
	t.Helper()
	parsed, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
	if err != nil {
		t.Fatalf("parsing %s: %v", path, err)
	}
	out := make([]string, 0, len(parsed.Imports))
	for _, spec := range parsed.Imports {
		out = append(out, strings.Trim(spec.Path.Value, `"`))
	}
	return out
}
