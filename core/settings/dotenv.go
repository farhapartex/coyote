package settings

import (
	"errors"
	"fmt"
	"io/fs"
	"os"

	"github.com/farhapartex/coyote/lib/dotenv"
)

type File struct {
	Path     string
	Optional bool
}

func DotEnv(path string) File { return File{Path: path, Optional: true} }

func RequiredDotEnv(path string) File { return File{Path: path} }

func (f File) Name() string { return f.Path }

func (f File) Values() (map[string]string, error) {
	handle, err := os.Open(f.Path)
	if err != nil {
		if f.Optional && errors.Is(err, fs.ErrNotExist) {
			return nil, nil
		}
		return nil, fmt.Errorf("coyote/settings: reading %s: %w", f.Path, err)
	}
	defer handle.Close()

	values, err := dotenv.Parse(handle)
	if err != nil {
		return nil, fmt.Errorf("coyote/settings: %s: %w", f.Path, err)
	}
	return values, nil
}

func LoadDotEnv(paths ...string) error {
	if len(paths) == 0 {
		paths = []string{".env"}
	}
	sources := make([]Source, 0, len(paths))
	for _, path := range paths {
		sources = append(sources, DotEnv(path))
	}
	return Load(sources...)
}

func MustLoadDotEnv(paths ...string) {
	if err := LoadDotEnv(paths...); err != nil {
		panic(err)
	}
}
