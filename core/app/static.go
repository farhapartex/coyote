package app

import (
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
)

const manifestName = "manifest.json"

type staticManifest struct {
	once    sync.Once
	entries map[string]string
}

func (a *App) StaticURL(name string) string {
	name = strings.TrimPrefix(name, "/")
	prefix := strings.TrimSuffix(a.Settings.Static.URL, "/")

	a.manifest.once.Do(func() {
		a.manifest.entries = loadManifest(a.Settings)
	})
	if stamped, found := a.manifest.entries[name]; found {
		return prefix + "/" + stamped
	}
	return prefix + "/" + name
}

func loadManifest(s Settings) map[string]string {
	if fsys := staticFS(s); fsys != nil {
		if file, err := fsys.Open(manifestName); err == nil {
			defer file.Close()
			if body, err := io.ReadAll(file); err == nil {
				return decodeManifest(body)
			}
		}
	}
	if s.Static.Dir != "" {
		if body, err := os.ReadFile(filepath.Join(s.Static.Dir, manifestName)); err == nil {
			return decodeManifest(body)
		}
	}
	return map[string]string{}
}

func decodeManifest(body []byte) map[string]string {
	entries := map[string]string{}
	if err := json.Unmarshal(body, &entries); err != nil {
		return map[string]string{}
	}
	return entries
}
