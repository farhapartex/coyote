package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

type FileSystem struct {
	root string
	perm os.FileMode
}

func NewFileSystem(root string) *FileSystem {
	return &FileSystem{root: root, perm: 0o750}
}

func (f *FileSystem) Root() string { return f.root }

func (f *FileSystem) path(key string) (string, error) {
	if err := ValidKey(key); err != nil {
		return "", err
	}
	full := filepath.Join(f.root, filepath.FromSlash(key))
	root, err := filepath.Abs(f.root)
	if err != nil {
		return "", err
	}
	resolved, err := filepath.Abs(full)
	if err != nil {
		return "", err
	}
	if resolved != root && !strings.HasPrefix(resolved, root+string(os.PathSeparator)) {
		return "", fmt.Errorf("%w: %q escapes the root", ErrBadKey, key)
	}
	return resolved, nil
}

func (f *FileSystem) Save(ctx context.Context, key string, r io.Reader) (Stat, error) {
	target, err := f.path(key)
	if err != nil {
		return Stat{}, err
	}
	if err := os.MkdirAll(filepath.Dir(target), f.perm); err != nil {
		return Stat{}, err
	}

	temp, err := os.CreateTemp(filepath.Dir(target), ".upload-*")
	if err != nil {
		return Stat{}, err
	}
	defer os.Remove(temp.Name())

	if _, err := io.Copy(temp, r); err != nil {
		temp.Close()
		return Stat{}, err
	}
	if err := temp.Close(); err != nil {
		return Stat{}, err
	}
	if err := os.Rename(temp.Name(), target); err != nil {
		return Stat{}, err
	}
	if err := os.Chmod(target, 0o640); err != nil {
		return Stat{}, err
	}
	return f.Stat(ctx, key)
}

func (f *FileSystem) Open(_ context.Context, key string) (io.ReadSeekCloser, error) {
	target, err := f.path(key)
	if err != nil {
		return nil, err
	}
	file, err := os.Open(target)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	return file, err
}

func (f *FileSystem) Stat(_ context.Context, key string) (Stat, error) {
	target, err := f.path(key)
	if err != nil {
		return Stat{}, err
	}
	info, err := os.Stat(target)
	if errors.Is(err, fs.ErrNotExist) {
		return Stat{}, fmt.Errorf("%w: %s", ErrNotFound, key)
	}
	if err != nil {
		return Stat{}, err
	}
	return Stat{Key: key, Size: info.Size(), Modified: info.ModTime()}, nil
}

func (f *FileSystem) Delete(_ context.Context, key string) error {
	target, err := f.path(key)
	if err != nil {
		return err
	}
	err = os.Remove(target)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	return err
}

func (f *FileSystem) Exists(ctx context.Context, key string) bool {
	_, err := f.Stat(ctx, key)
	return err == nil
}

func (f *FileSystem) Move(_ context.Context, from, to string) error {
	source, err := f.path(from)
	if err != nil {
		return err
	}
	target, err := f.path(to)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(filepath.Dir(target), f.perm); err != nil {
		return err
	}
	err = os.Rename(source, target)
	if errors.Is(err, fs.ErrNotExist) {
		return fmt.Errorf("%w: %s", ErrNotFound, from)
	}
	return err
}

func (f *FileSystem) List(_ context.Context, prefix string) ([]Stat, error) {
	base := f.root
	if prefix != "" {
		resolved, err := f.path(prefix)
		if err != nil {
			return nil, err
		}
		base = resolved
	}

	out := []Stat{}
	err := filepath.WalkDir(base, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if entry.IsDir() {
			return nil
		}
		info, err := entry.Info()
		if err != nil {
			return err
		}
		relative, err := filepath.Rel(f.root, path)
		if err != nil {
			return err
		}
		out = append(out, Stat{
			Key:      filepath.ToSlash(relative),
			Size:     info.Size(),
			Modified: info.ModTime(),
		})
		return nil
	})
	return out, err
}
