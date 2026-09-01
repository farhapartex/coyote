package upload

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/farhapartex/coyote/core/storage"
)

var (
	ErrNoFile       = errors.New("coyote/upload: no file in the request")
	ErrNotConfigued = errors.New("coyote/upload: no storage configured")
)

type File struct {
	Ref      Ref
	Name     string
	Type     string
	Size     int64
	Digest   string
	Uploaded time.Time
}

type Options struct {
	Storage  storage.Storage
	Rules    Rules
	Path     string
	TrashTTL time.Duration
	StageTTL time.Duration
}

type Service struct {
	store    storage.Storage
	rules    Rules
	path     string
	trashTTL time.Duration
	stageTTL time.Duration
}

func New(opts Options) *Service {
	if opts.StageTTL <= 0 {
		opts.StageTTL = 24 * time.Hour
	}
	return &Service{
		store:    opts.Storage,
		rules:    opts.Rules,
		path:     CleanPath(opts.Path),
		trashTTL: opts.TrashTTL,
		stageTTL: opts.StageTTL,
	}
}

func (s *Service) Storage() storage.Storage { return s.store }

func (s *Service) Rules() Rules { return s.rules }

func (s *Service) Path() string { return s.path }

func (s *Service) PathFor(field string) string {
	if cleaned := CleanPath(field); cleaned != "" {
		return cleaned
	}
	return s.path
}

func (s *Service) Accept(r *http.Request, field string) (File, error) {
	return s.AcceptTo(r, field, "")
}

func (s *Service) AcceptTo(r *http.Request, field, path string) (File, error) {
	if s.store == nil {
		return File{}, ErrNotConfigued
	}
	if s.rules.MaxSize > 0 {
		r.Body = http.MaxBytesReader(nil, r.Body, s.rules.MaxSize+multipartSlack)
	}
	if err := r.ParseMultipartForm(readBuffer); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return File{}, fmt.Errorf("%w: limit is %d bytes", ErrTooLarge, s.rules.MaxSize)
		}
		return File{}, err
	}

	file, header, err := r.FormFile(field)
	if err != nil {
		if errors.Is(err, http.ErrMissingFile) {
			return File{}, fmt.Errorf("%w: %s", ErrNoFile, field)
		}
		return File{}, err
	}
	defer file.Close()

	return s.StoreTo(r.Context(), file, header.Filename, header.Header.Get("Content-Type"), header.Size, path)
}

const (
	readBuffer     = 8 << 20
	multipartSlack = 4 << 10
)

func (s *Service) Store(ctx context.Context, r io.Reader, name, declared string, size int64) (File, error) {
	return s.StoreTo(ctx, r, name, declared, size, "")
}

func (s *Service) StoreTo(ctx context.Context, r io.Reader, name, declared string, size int64, path string) (File, error) {
	if s.store == nil {
		return File{}, ErrNotConfigued
	}
	if s.rules.MaxSize > 0 && size > s.rules.MaxSize {
		return File{}, fmt.Errorf("%w: %d bytes", ErrTooLarge, size)
	}

	head := make([]byte, sniffLength)
	read, err := io.ReadFull(r, head)
	if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
		return File{}, err
	}
	head = head[:read]

	kind, err := s.rules.Check(head, declared)
	if err != nil {
		return File{}, err
	}

	body := io.MultiReader(strings.NewReader(string(head)), r)
	if s.rules.MaxSize > 0 {
		body = io.LimitReader(body, s.rules.MaxSize+1)
	}

	temp, err := os.CreateTemp("", "coyote-upload-*")
	if err != nil {
		return File{}, err
	}
	defer os.Remove(temp.Name())
	defer temp.Close()

	digest := sha256.New()
	written, err := io.Copy(io.MultiWriter(temp, digest), body)
	if err != nil {
		return File{}, err
	}
	if s.rules.MaxSize > 0 && written > s.rules.MaxSize {
		return File{}, fmt.Errorf("%w: limit is %d bytes", ErrTooLarge, s.rules.MaxSize)
	}
	if written == 0 {
		return File{}, ErrEmptyFile
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		return File{}, err
	}
	if err := s.rules.CheckPixels(temp, kind); err != nil {
		return File{}, err
	}
	if _, err := temp.Seek(0, io.SeekStart); err != nil {
		return File{}, err
	}

	sum := hex.EncodeToString(digest.Sum(nil))
	ref := Ref(join(StagedPrefix, s.PathFor(path), fmt.Sprintf("%s/%s/%s%s", sum[:2], sum[2:4], sum, ExtensionFor(kind))))

	if _, err := s.store.Save(ctx, string(ref), temp); err != nil {
		return File{}, err
	}

	return File{
		Ref:      ref,
		Name:     SafeName(name),
		Type:     kind,
		Size:     written,
		Digest:   sum,
		Uploaded: time.Now(),
	}, nil
}

func (s *Service) Commit(ctx context.Context, refs ...*Ref) error {
	for _, ref := range refs {
		if ref == nil || ref.Empty() || !ref.Staged() {
			continue
		}
		target := ref.promoted()
		if s.store.Exists(ctx, string(target)) {
			if err := s.store.Delete(ctx, string(*ref)); err != nil {
				return err
			}
			*ref = target
			continue
		}
		if err := s.store.Move(ctx, string(*ref), string(target)); err != nil {
			return err
		}
		*ref = target
	}
	return nil
}

func (s *Service) Delete(ctx context.Context, ref Ref) error {
	if ref.Empty() {
		return nil
	}
	if s.trashTTL <= 0 {
		return s.store.Delete(ctx, string(ref))
	}
	if !ref.Committed() {
		return s.store.Delete(ctx, string(ref))
	}
	return s.store.Move(ctx, string(ref), string(ref.intoTrash()))
}

func (s *Service) Open(ctx context.Context, ref Ref) (io.ReadSeekCloser, error) {
	if ref.Empty() {
		return nil, storage.ErrNotFound
	}
	return s.store.Open(ctx, string(ref))
}

func (s *Service) Sweep(ctx context.Context) (staged int, trashed int, err error) {
	now := time.Now()

	stagedFiles, err := s.store.List(ctx, StagedPrefix)
	if err != nil {
		return 0, 0, err
	}
	for _, stat := range stagedFiles {
		if now.Sub(stat.Modified) < s.stageTTL {
			continue
		}
		if err := s.store.Delete(ctx, stat.Key); err != nil {
			return staged, trashed, err
		}
		staged++
	}

	if s.trashTTL <= 0 {
		return staged, 0, nil
	}
	trashFiles, err := s.store.List(ctx, TrashPrefix)
	if err != nil {
		return staged, 0, err
	}
	for _, stat := range trashFiles {
		if now.Sub(stat.Modified) < s.trashTTL {
			continue
		}
		if err := s.store.Delete(ctx, stat.Key); err != nil {
			return staged, trashed, err
		}
		trashed++
	}
	return staged, trashed, nil
}

func (s *Service) Each(r *http.Request, fn func(part *multipart.Part) error) error {
	reader, err := r.MultipartReader()
	if err != nil {
		return err
	}
	for {
		part, err := reader.NextPart()
		if errors.Is(err, io.EOF) {
			return nil
		}
		if err != nil {
			return err
		}
		if err := fn(part); err != nil {
			part.Close()
			return err
		}
		part.Close()
	}
}
