package i18n

import (
	"fmt"
	"io/fs"
	"path"
)

type FSLoader struct {
	FS  fs.FS
	Dir string
}

func NewFSLoader(fsys fs.FS, dir string) *FSLoader {
	return &FSLoader{FS: fsys, Dir: dir}
}

func (l *FSLoader) Load(tag string) (Catalog, error) {
	if l.FS == nil {
		return nil, fmt.Errorf("%w: %s", ErrNoCatalog, tag)
	}

	for _, candidate := range l.candidates(tag) {
		file, err := l.FS.Open(candidate)
		if err != nil {
			continue
		}
		catalog, err := ParsePO(tag, file)
		file.Close()
		if err != nil {
			return nil, fmt.Errorf("reading %s: %w", candidate, err)
		}
		return catalog, nil
	}
	return nil, fmt.Errorf("%w: %s", ErrNoCatalog, tag)
}

func (l *FSLoader) candidates(tag string) []string {
	normalised := Normalise(tag)
	names := []string{normalised + Extension}
	if base := BaseOf(tag); base != normalised {
		names = append(names, base+Extension)
	}

	out := make([]string, 0, len(names)*2)
	for _, name := range names {
		if l.Dir != "" {
			out = append(out, path.Join(l.Dir, name))
		}
		out = append(out, name)
	}
	return out
}

func LoadInto(bundle *Bundle, loader Loader) []error {
	if bundle == nil || loader == nil {
		return nil
	}

	problems := []error{}
	for _, tag := range bundle.Supported() {
		catalog, err := loader.Load(tag)
		if err != nil {
			problems = append(problems, err)
			continue
		}
		bundle.Add(catalog)
	}
	return problems
}

func Layer(bundle *Bundle, fsys fs.FS, dir string) []error {
	if bundle == nil || fsys == nil {
		return nil
	}
	return LoadInto(bundle, NewFSLoader(fsys, dir))
}
