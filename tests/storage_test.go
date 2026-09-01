package tests

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/farhapartex/coyote/core/storage"
)

func newStore(t *testing.T) (*storage.FileSystem, context.Context) {
	t.Helper()
	return storage.NewFileSystem(filepath.Join(t.TempDir(), "media")), context.Background()
}

func TestFileSystemRoundTrip(t *testing.T) {
	store, ctx := newStore(t)

	stat, err := store.Save(ctx, "ab/cd/file.txt", strings.NewReader("hello world"))
	if err != nil {
		t.Fatal(err)
	}
	if stat.Size != 11 || stat.Key != "ab/cd/file.txt" {
		t.Errorf("stat = %+v", stat)
	}
	if !store.Exists(ctx, "ab/cd/file.txt") {
		t.Error("the object should exist after saving")
	}

	reader, err := store.Open(ctx, "ab/cd/file.txt")
	if err != nil {
		t.Fatal(err)
	}
	defer reader.Close()
	body := make([]byte, 11)
	if _, err := reader.Read(body); err != nil {
		t.Fatal(err)
	}
	if string(body) != "hello world" {
		t.Errorf("body = %q", body)
	}

	if err := store.Delete(ctx, "ab/cd/file.txt"); err != nil {
		t.Fatal(err)
	}
	if store.Exists(ctx, "ab/cd/file.txt") {
		t.Error("the object should be gone")
	}
	if err := store.Delete(ctx, "ab/cd/file.txt"); err != nil {
		t.Errorf("deleting twice should be quiet, got %v", err)
	}
}

func TestMissingObjectsReportNotFound(t *testing.T) {
	store, ctx := newStore(t)

	if _, err := store.Open(ctx, "nope.txt"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Open error = %v, want ErrNotFound", err)
	}
	if _, err := store.Stat(ctx, "nope.txt"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Stat error = %v, want ErrNotFound", err)
	}
	if err := store.Move(ctx, "nope.txt", "other.txt"); !errors.Is(err, storage.ErrNotFound) {
		t.Errorf("Move error = %v, want ErrNotFound", err)
	}
}

func TestKeysCannotEscapeTheRoot(t *testing.T) {
	store, ctx := newStore(t)

	for _, key := range []string{
		"../escape.txt",
		"a/../../escape.txt",
		"/absolute.txt",
		`windows\path.txt`,
		"",
		"a//b.txt",
		"./relative.txt",
		"trailing/",
		"nul\x00byte.txt",
	} {
		if _, err := store.Save(ctx, key, strings.NewReader("x")); !errors.Is(err, storage.ErrBadKey) {
			t.Errorf("Save(%q) error = %v, want ErrBadKey", key, err)
		}
		if _, err := store.Open(ctx, key); !errors.Is(err, storage.ErrBadKey) {
			t.Errorf("Open(%q) error = %v, want ErrBadKey", key, err)
		}
	}
}

func TestValidKeyAcceptsWhatWeGenerate(t *testing.T) {
	for _, key := range []string{
		"ab/cd/abcdef.png",
		"staged/ab/cd/abcdef.png",
		"file.txt",
		"a/b/c/d/e.bin",
	} {
		if err := storage.ValidKey(key); err != nil {
			t.Errorf("ValidKey(%q) = %v, want nil", key, err)
		}
	}
}

func TestMovePromotesBetweenAreas(t *testing.T) {
	store, ctx := newStore(t)

	if _, err := store.Save(ctx, "staged/ab/file.bin", strings.NewReader("data")); err != nil {
		t.Fatal(err)
	}
	if err := store.Move(ctx, "staged/ab/file.bin", "media/ab/file.bin"); err != nil {
		t.Fatal(err)
	}

	if store.Exists(ctx, "staged/ab/file.bin") {
		t.Error("the staged copy should be gone")
	}
	if !store.Exists(ctx, "media/ab/file.bin") {
		t.Error("the promoted copy should exist")
	}
}

func TestListWalksAPrefix(t *testing.T) {
	store, ctx := newStore(t)

	for _, key := range []string{"staged/one.txt", "staged/two.txt", "media/three.txt"} {
		if _, err := store.Save(ctx, key, strings.NewReader("x")); err != nil {
			t.Fatal(err)
		}
	}

	staged, err := store.List(ctx, "staged")
	if err != nil {
		t.Fatal(err)
	}
	if len(staged) != 2 {
		t.Errorf("staged = %+v", staged)
	}
	for _, stat := range staged {
		if !strings.HasPrefix(stat.Key, "staged/") {
			t.Errorf("key %q is outside the prefix", stat.Key)
		}
	}

	all, err := store.List(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(all) != 3 {
		t.Errorf("all = %+v", all)
	}

	missing, err := store.List(ctx, "absent")
	if err != nil {
		t.Fatal(err)
	}
	if len(missing) != 0 {
		t.Errorf("an absent prefix should list nothing, got %+v", missing)
	}
}

func TestSavesAreAtomic(t *testing.T) {
	store, ctx := newStore(t)

	if _, err := store.Save(ctx, "file.txt", strings.NewReader("first")); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Save(ctx, "file.txt", strings.NewReader("second value")); err != nil {
		t.Fatal(err)
	}

	stat, err := store.Stat(ctx, "file.txt")
	if err != nil {
		t.Fatal(err)
	}
	if stat.Size != 12 {
		t.Errorf("size = %d, want the second write", stat.Size)
	}

	listed, err := store.List(ctx, "")
	if err != nil {
		t.Fatal(err)
	}
	if len(listed) != 1 {
		t.Errorf("a replaced file should leave no temporary behind: %+v", listed)
	}
}

func TestASymlinkCannotLeadOutOfTheRoot(t *testing.T) {
	base := t.TempDir()
	root := filepath.Join(base, "media")
	outside := filepath.Join(base, "outside")
	for _, dir := range []string{root, outside} {
		if err := os.MkdirAll(dir, 0o750); err != nil {
			t.Fatal(err)
		}
	}
	if err := os.WriteFile(filepath.Join(outside, "secret.txt"), []byte("not yours"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.Symlink(outside, filepath.Join(root, "escape")); err != nil {
		t.Skipf("symlinks are unavailable here: %v", err)
	}

	store := storage.NewFileSystem(root)
	ctx := t.Context()

	if _, err := store.Open(ctx, "escape/secret.txt"); !errors.Is(err, storage.ErrBadKey) {
		t.Errorf("error = %v, want ErrBadKey; a link inside the root led outside it", err)
	}
	if _, err := store.Save(ctx, "escape/planted.txt", strings.NewReader("x")); !errors.Is(err, storage.ErrBadKey) {
		t.Errorf("error = %v, want ErrBadKey; a write followed the link out", err)
	}
	if _, err := os.Stat(filepath.Join(outside, "planted.txt")); err == nil {
		t.Error("a file was written outside the media root")
	}
}

func TestAnOrdinaryPathIsStillReachableThroughASymlinkedRoot(t *testing.T) {
	base := t.TempDir()
	real := filepath.Join(base, "real")
	if err := os.MkdirAll(real, 0o750); err != nil {
		t.Fatal(err)
	}
	linked := filepath.Join(base, "linked")
	if err := os.Symlink(real, linked); err != nil {
		t.Skipf("symlinks are unavailable here: %v", err)
	}

	store := storage.NewFileSystem(linked)
	if _, err := store.Save(t.Context(), "ab/file.txt", strings.NewReader("hello")); err != nil {
		t.Errorf("a root that is itself a symlink should still work: %v", err)
	}
}
