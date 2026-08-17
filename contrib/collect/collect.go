package collect

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"sort"
	"strings"
)

const ManifestName = "manifest.json"

type Result struct {
	Dir     string
	Entries map[string]string
	Skipped []string
}

func (r Result) Names() []string {
	out := make([]string, 0, len(r.Entries))
	for name := range r.Entries {
		out = append(out, name)
	}
	sort.Strings(out)
	return out
}

func Run(source fs.FS, target string) (Result, error) {
	result := Result{Dir: target, Entries: map[string]string{}}
	if source == nil {
		return result, fmt.Errorf("coyote/collect: no static files configured")
	}
	if err := os.MkdirAll(target, 0o755); err != nil {
		return result, err
	}

	err := fs.WalkDir(source, ".", func(name string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() || strings.HasPrefix(path.Base(name), ".") {
			return nil
		}

		body, err := fs.ReadFile(source, name)
		if err != nil {
			return err
		}
		digest := sha256.Sum256(body)
		stamped := Fingerprint(name, hex.EncodeToString(digest[:])[:12])

		written := filepath.Join(target, filepath.FromSlash(stamped))
		if err := os.MkdirAll(filepath.Dir(written), 0o755); err != nil {
			return err
		}
		if err := os.WriteFile(written, body, 0o644); err != nil {
			return err
		}
		result.Entries[name] = stamped
		return nil
	})
	if err != nil {
		return result, err
	}

	return result, writeManifest(target, result.Entries)
}

func Fingerprint(name, digest string) string {
	extension := path.Ext(name)
	return strings.TrimSuffix(name, extension) + "." + digest + extension
}

func writeManifest(target string, entries map[string]string) error {
	body, err := json.MarshalIndent(entries, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(target, ManifestName), append(body, '\n'), 0o644)
}

func LoadManifest(dir string) (map[string]string, error) {
	body, err := os.ReadFile(filepath.Join(dir, ManifestName))
	if err != nil {
		return nil, err
	}
	entries := map[string]string{}
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}

func ManifestFrom(source fs.FS) (map[string]string, error) {
	file, err := source.Open(ManifestName)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	body, err := io.ReadAll(file)
	if err != nil {
		return nil, err
	}
	entries := map[string]string{}
	if err := json.Unmarshal(body, &entries); err != nil {
		return nil, err
	}
	return entries, nil
}
