package storage

import (
	"fmt"
	"path"
	"strings"
)

func ValidKey(key string) error {
	if key == "" {
		return fmt.Errorf("%w: empty", ErrBadKey)
	}
	if len(key) > 512 {
		return fmt.Errorf("%w: longer than 512 characters", ErrBadKey)
	}
	if strings.ContainsRune(key, 0) {
		return fmt.Errorf("%w: contains a null byte", ErrBadKey)
	}
	if strings.HasPrefix(key, "/") || strings.HasPrefix(key, `\`) {
		return fmt.Errorf("%w: must be relative", ErrBadKey)
	}
	if strings.Contains(key, `\`) {
		return fmt.Errorf("%w: use forward slashes", ErrBadKey)
	}
	if key != path.Clean(key) {
		return fmt.Errorf("%w: %q is not a clean path", ErrBadKey, key)
	}
	for _, segment := range strings.Split(key, "/") {
		if segment == "" || segment == "." || segment == ".." {
			return fmt.Errorf("%w: %q escapes the root", ErrBadKey, key)
		}
	}
	return nil
}
