package messages

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

var skippedDirs = map[string]bool{
	".git": true, "node_modules": true, "vendor": true, "staticfiles": true,
	"media": true, "cache": true, "locales": true, "migrations": true,
}

var templateExtensions = map[string]bool{
	".html": true, ".tmpl": true, ".gohtml": true, ".txt": true,
}

type Scan struct {
	Messages *Set
	Problems []Problem
	Files    int
}

func Walk(root string) (Scan, error) {
	out := Scan{Messages: NewSet()}

	err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if path != root && (skippedDirs[entry.Name()] || strings.HasPrefix(entry.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}

		relative, relErr := filepath.Rel(root, path)
		if relErr != nil {
			relative = path
		}
		relative = filepath.ToSlash(relative)

		extension := strings.ToLower(filepath.Ext(path))
		switch {
		case extension == ".go":
			if strings.HasSuffix(path, "_test.go") {
				return nil
			}
			return out.readGo(path, relative)
		case templateExtensions[extension]:
			return out.readTemplate(path, relative)
		}
		return nil
	})
	return out, err
}

func (s *Scan) readGo(path, relative string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	found, problems, err := FromGo(relative, source)
	if err != nil {
		s.Problems = append(s.Problems, Problem{File: relative, Reason: err.Error()})
		return nil
	}
	s.absorb(found, problems)
	return nil
}

func (s *Scan) readTemplate(path, relative string) error {
	source, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	found, problems, err := FromTemplate(relative, string(source))
	if err != nil {
		s.Problems = append(s.Problems, Problem{File: relative, Reason: err.Error()})
		return nil
	}
	s.absorb(found, problems)
	return nil
}

func (s *Scan) absorb(found *Set, problems []Problem) {
	s.Files++
	for _, message := range found.All() {
		s.Messages.Add(message)
	}
	s.Problems = append(s.Problems, problems...)
}
