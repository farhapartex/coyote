package cache

import (
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	expiryBytes = 8
	lengthBytes = 4
	headerSize  = expiryBytes + lengthBytes
	fileSuffix  = ".cache"
)

var errCorruptEntry = errors.New("coyote/cache: cached file is truncated or corrupt")

type storedEntry struct {
	key     string
	value   []byte
	expires int64
}

func (e storedEntry) expired(now int64) bool {
	return e.expires > 0 && e.expires < now
}

func encodeEntry(key string, value []byte, ttl time.Duration) []byte {
	body := make([]byte, headerSize+len(key)+len(value))
	if ttl > 0 {
		binary.BigEndian.PutUint64(body[:expiryBytes], uint64(time.Now().Add(ttl).UnixMilli()))
	}
	binary.BigEndian.PutUint32(body[expiryBytes:headerSize], uint32(len(key)))
	copy(body[headerSize:], key)
	copy(body[headerSize+len(key):], value)
	return body
}

func decodeEntry(raw []byte) (storedEntry, error) {
	if len(raw) < headerSize {
		return storedEntry{}, errCorruptEntry
	}
	expires := int64(binary.BigEndian.Uint64(raw[:expiryBytes]))
	length := int(binary.BigEndian.Uint32(raw[expiryBytes:headerSize]))
	if length < 0 || headerSize+length > len(raw) {
		return storedEntry{}, errCorruptEntry
	}
	return storedEntry{
		key:     string(raw[headerSize : headerSize+length]),
		value:   raw[headerSize+length:],
		expires: expires,
	}, nil
}

func entryPath(dir, key string) string {
	digest := sha256.Sum256([]byte(key))
	name := hex.EncodeToString(digest[:])
	return filepath.Join(dir, name[:2], name[2:4], name+fileSuffix)
}

func readEntry(path string) (storedEntry, error) {
	raw, err := os.ReadFile(path)
	if err != nil {
		return storedEntry{}, err
	}
	return decodeEntry(raw)
}

func walkEntries(dir string, fn func(path string, entry storedEntry) error) error {
	return filepath.WalkDir(dir, func(path string, item fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if item.IsDir() || !strings.HasSuffix(item.Name(), fileSuffix) {
			return nil
		}
		entry, err := readEntry(path)
		if err != nil {
			return nil
		}
		return fn(path, entry)
	})
}

func writeEntry(path string, body []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o750); err != nil {
		return fmt.Errorf("coyote/cache: creating %s: %w", directory, err)
	}

	temp, err := os.CreateTemp(directory, ".cache-*")
	if err != nil {
		return err
	}
	defer os.Remove(temp.Name())

	if _, err := temp.Write(body); err != nil {
		temp.Close()
		return err
	}
	if err := temp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(temp.Name(), 0o640); err != nil {
		return err
	}
	return os.Rename(temp.Name(), path)
}
