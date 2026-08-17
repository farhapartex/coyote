package tests

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"testing/fstest"

	"github.com/farhapartex/coyote/contrib/collect"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/settings"
)

func staticSource() fstest.MapFS {
	return fstest.MapFS{
		"site.css":     &fstest.MapFile{Data: []byte("body{color:#00ADD8}")},
		"js/app.js":    &fstest.MapFile{Data: []byte("console.log(1)")},
		".hidden":      &fstest.MapFile{Data: []byte("skip me")},
		"img/logo.png": &fstest.MapFile{Data: []byte("not really a png")},
	}
}

func TestCollectFingerprintsEveryFile(t *testing.T) {
	target := filepath.Join(t.TempDir(), "staticfiles")

	result, err := collect.Run(staticSource(), target)
	if err != nil {
		t.Fatal(err)
	}

	if len(result.Entries) != 3 {
		t.Errorf("collected %v, want three files and no dotfile", result.Names())
	}
	stamped, found := result.Entries["site.css"]
	if !found {
		t.Fatal("site.css was not collected")
	}
	if !strings.HasPrefix(stamped, "site.") || !strings.HasSuffix(stamped, ".css") {
		t.Errorf("fingerprinted name = %q", stamped)
	}
	if stamped == "site.css" {
		t.Error("the name should carry a digest")
	}
	if _, err := os.Stat(filepath.Join(target, filepath.FromSlash(stamped))); err != nil {
		t.Errorf("the stamped file should exist: %v", err)
	}
	if _, err := os.Stat(filepath.Join(target, "js", filepath.Base(result.Entries["js/app.js"]))); err != nil {
		t.Errorf("nested files should keep their directory: %v", err)
	}
}

func TestCollectIsDeterministicAndContentDriven(t *testing.T) {
	first, err := collect.Run(staticSource(), filepath.Join(t.TempDir(), "a"))
	if err != nil {
		t.Fatal(err)
	}
	second, err := collect.Run(staticSource(), filepath.Join(t.TempDir(), "b"))
	if err != nil {
		t.Fatal(err)
	}
	if first.Entries["site.css"] != second.Entries["site.css"] {
		t.Error("the same bytes should fingerprint the same way")
	}

	changed := staticSource()
	changed["site.css"] = &fstest.MapFile{Data: []byte("body{color:red}")}
	third, err := collect.Run(changed, filepath.Join(t.TempDir(), "c"))
	if err != nil {
		t.Fatal(err)
	}
	if third.Entries["site.css"] == first.Entries["site.css"] {
		t.Error("changed bytes must change the name, or caches would serve the old file")
	}
}

func TestCollectWritesAReadableManifest(t *testing.T) {
	target := filepath.Join(t.TempDir(), "staticfiles")
	result, err := collect.Run(staticSource(), target)
	if err != nil {
		t.Fatal(err)
	}

	entries, err := collect.LoadManifest(target)
	if err != nil {
		t.Fatal(err)
	}
	if entries["site.css"] != result.Entries["site.css"] {
		t.Errorf("manifest = %v", entries)
	}
}

func TestStaticURLUsesTheManifestWhenPresent(t *testing.T) {
	source := staticSource()
	stamped := "site.abc123def456.css"
	source[collect.ManifestName] = &fstest.MapFile{Data: []byte(`{"site.css":"` + stamped + `"}`)}

	a := app.NewFrom(devSettings(t, func(s *settings.Settings) {
		s.Static.FS = source
		s.Static.URL = "/static/"
	}))

	if got := a.StaticURL("site.css"); got != "/static/"+stamped {
		t.Errorf("StaticURL = %q, want the fingerprinted path", got)
	}
	if got := a.StaticURL("js/app.js"); got != "/static/js/app.js" {
		t.Errorf("an unlisted file should fall back to its plain path, got %q", got)
	}
}

func TestStaticURLFallsBackWithoutAManifest(t *testing.T) {
	a := app.NewFrom(devSettings(t, func(s *settings.Settings) {
		s.Static.FS = staticSource()
		s.Static.URL = "/static/"
	}))

	if got := a.StaticURL("site.css"); got != "/static/site.css" {
		t.Errorf("StaticURL = %q, want the plain path in development", got)
	}
	if got := a.StaticURL("/site.css"); got != "/static/site.css" {
		t.Errorf("a leading slash should not double up: %q", got)
	}
}

func TestCollectRefusesWithoutASource(t *testing.T) {
	if _, err := collect.Run(nil, t.TempDir()); err == nil {
		t.Error("collecting nothing should be an error")
	}
}
