package mail

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"
)

const (
	fileMode = 0o600
	dirMode  = 0o700
)

type FileSender struct {
	dir     string
	encoder Encoder
}

func NewFile(dir string) *FileSender {
	return &FileSender{dir: dir, encoder: NewEncoder()}
}

func (f *FileSender) Dir() string {
	return f.dir
}

func (f *FileSender) Send(ctx context.Context, message Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	raw, err := f.encoder.Encode(message)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(f.dir, dirMode); err != nil {
		return fmt.Errorf("%w: creating %s: %w", ErrSendFailed, f.dir, err)
	}

	temporary, err := os.CreateTemp(f.dir, ".coyote-mail-*")
	if err != nil {
		return fmt.Errorf("%w: creating a temporary file in %s: %w", ErrSendFailed, f.dir, err)
	}
	name := temporary.Name()

	if err := f.write(temporary, raw); err != nil {
		os.Remove(name)
		return err
	}

	final := filepath.Join(f.dir, fmt.Sprintf("%d-%s.eml", time.Now().UnixNano(), randomToken()))
	if err := os.Rename(name, final); err != nil {
		os.Remove(name)
		return fmt.Errorf("%w: naming %s: %w", ErrSendFailed, final, err)
	}
	return nil
}

func (f *FileSender) write(file *os.File, raw []byte) error {
	if err := file.Chmod(fileMode); err != nil {
		file.Close()
		return fmt.Errorf("%w: setting permissions on %s: %w", ErrSendFailed, file.Name(), err)
	}
	if _, err := file.Write(raw); err != nil {
		file.Close()
		return fmt.Errorf("%w: writing %s: %w", ErrSendFailed, file.Name(), err)
	}
	if err := file.Close(); err != nil {
		return fmt.Errorf("%w: closing %s: %w", ErrSendFailed, file.Name(), err)
	}
	return nil
}
