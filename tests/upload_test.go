package tests

import (
	"bytes"
	"context"
	"errors"
	"image"
	"image/color"
	"image/png"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/storage"
	"github.com/farhapartex/coyote/core/upload"
)

func uploads(t *testing.T, rules upload.Rules, trash time.Duration) (*upload.Service, context.Context) {
	t.Helper()
	if len(rules.Allowed) == 0 {
		rules.Allowed = []string{"image/png", "application/pdf", "text/plain"}
	}
	service := upload.New(upload.Options{
		Storage:  storage.NewFileSystem(filepath.Join(t.TempDir(), "media")),
		Rules:    rules,
		TrashTTL: trash,
	})
	return service, context.Background()
}

func pngBytes(t *testing.T, width, height int) []byte {
	t.Helper()
	canvas := image.NewRGBA(image.Rect(0, 0, width, height))
	canvas.Set(0, 0, color.RGBA{R: 255, A: 255})
	buffer := &bytes.Buffer{}
	if err := png.Encode(buffer, canvas); err != nil {
		t.Fatal(err)
	}
	return buffer.Bytes()
}

func TestStoreStagesAContentAddressedFile(t *testing.T) {
	service, ctx := uploads(t, upload.Rules{}, 0)
	body := pngBytes(t, 4, 4)

	file, err := service.Store(ctx, bytes.NewReader(body), "holiday photo.png", "image/png", int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}

	if !file.Ref.Staged() {
		t.Errorf("a new upload should land in staging, got %q", file.Ref)
	}
	if file.Type != "image/png" || file.Size != int64(len(body)) {
		t.Errorf("file = %+v", file)
	}
	if !strings.HasSuffix(string(file.Ref), ".png") {
		t.Errorf("the extension should come from the sniffed type, got %q", file.Ref)
	}
	if !strings.Contains(string(file.Ref), file.Digest) {
		t.Errorf("the key should be the digest: %q vs %q", file.Ref, file.Digest)
	}
	if !service.Storage().Exists(ctx, string(file.Ref)) {
		t.Error("the bytes should be in storage")
	}

	again, err := service.Store(ctx, bytes.NewReader(body), "copy.png", "image/png", int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if again.Ref != file.Ref {
		t.Errorf("identical bytes should share a key: %q vs %q", again.Ref, file.Ref)
	}
}

func TestUploadRejectsDisallowedTypes(t *testing.T) {
	service, ctx := uploads(t, upload.Rules{Allowed: []string{"image/png"}}, 0)

	_, err := service.Store(ctx, strings.NewReader("%PDF-1.4 not really"), "doc.pdf", "application/pdf", 19)
	if !errors.Is(err, upload.ErrTypeNotAllow) {
		t.Errorf("error = %v, want ErrTypeNotAllow", err)
	}
}

func TestUploadRejectsATypeThatDoesNotMatchTheContent(t *testing.T) {
	service, ctx := uploads(t, upload.Rules{Allowed: []string{"image/png", "application/pdf"}}, 0)
	body := pngBytes(t, 2, 2)

	_, err := service.Store(ctx, bytes.NewReader(body), "invoice.pdf", "application/pdf", int64(len(body)))
	if !errors.Is(err, upload.ErrTypeMismatch) {
		t.Errorf("error = %v, want ErrTypeMismatch", err)
	}
}

func TestUploadIgnoresAMaliciousFilename(t *testing.T) {
	service, ctx := uploads(t, upload.Rules{}, 0)
	body := pngBytes(t, 2, 2)

	file, err := service.Store(ctx, bytes.NewReader(body), "../../../etc/passwd.png", "image/png", int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(file.Ref), "..") || strings.Contains(string(file.Ref), "passwd") {
		t.Errorf("the stored key must not come from the filename, got %q", file.Ref)
	}
	if file.Name != "passwd.png" {
		t.Errorf("the display name should be the sanitised base, got %q", file.Name)
	}
}

func TestSafeNameHandlesHostileInput(t *testing.T) {
	for input, want := range map[string]string{
		"../../etc/passwd":    "passwd",
		`C:\Windows\evil.exe`: "evil.exe",
		"  spaced .txt  ":     "spaced .txt",
		"":                    "file",
		"..":                  "file",
		"con.txt":             "_con.txt",
		"nul\x00.png":         "_nul.png",
		"photo\x00.png":       "photo.png",
		"a/b/c/report.pdf":    "report.pdf",
	} {
		if got := upload.SafeName(input); got != want {
			t.Errorf("SafeName(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestUploadRefusesOversizeContent(t *testing.T) {
	service, ctx := uploads(t, upload.Rules{MaxSize: 64, Allowed: []string{"text/plain"}}, 0)

	_, err := service.Store(ctx, strings.NewReader(strings.Repeat("a", 500)), "big.txt", "", 500)
	if !errors.Is(err, upload.ErrTooLarge) {
		t.Errorf("error = %v, want ErrTooLarge", err)
	}

	_, err = service.Store(ctx, strings.NewReader(strings.Repeat("a", 500)), "big.txt", "", 0)
	if !errors.Is(err, upload.ErrTooLarge) {
		t.Errorf("a lying size header must not get past the limit: %v", err)
	}
}

func TestUploadRefusesAnEmptyFile(t *testing.T) {
	service, ctx := uploads(t, upload.Rules{}, 0)

	if _, err := service.Store(ctx, strings.NewReader(""), "empty.txt", "", 0); !errors.Is(err, upload.ErrEmptyFile) {
		t.Errorf("error = %v, want ErrEmptyFile", err)
	}
}

func TestUploadRefusesAnOversizedImage(t *testing.T) {
	service, ctx := uploads(t, upload.Rules{Allowed: []string{"image/png"}, MaxPixels: 16}, 0)
	body := pngBytes(t, 100, 100)

	_, err := service.Store(ctx, bytes.NewReader(body), "huge.png", "image/png", int64(len(body)))
	if !errors.Is(err, upload.ErrTooManyPixel) {
		t.Errorf("error = %v, want ErrTooManyPixel", err)
	}
}

func TestCommitPromotesAndDeleteRemoves(t *testing.T) {
	service, ctx := uploads(t, upload.Rules{}, 0)
	body := pngBytes(t, 2, 2)

	file, err := service.Store(ctx, bytes.NewReader(body), "a.png", "image/png", int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	staged := file.Ref

	ref := file.Ref
	if err := service.Commit(ctx, &ref); err != nil {
		t.Fatal(err)
	}
	if !ref.Committed() {
		t.Errorf("ref = %q, want a committed key", ref)
	}
	if service.Storage().Exists(ctx, string(staged)) {
		t.Error("the staged copy should be gone after committing")
	}
	if !service.Storage().Exists(ctx, string(ref)) {
		t.Error("the committed copy should exist")
	}

	if err := service.Delete(ctx, ref); err != nil {
		t.Fatal(err)
	}
	if service.Storage().Exists(ctx, string(ref)) {
		t.Error("with TrashTTL zero the bytes should go immediately")
	}
}

func TestDeleteKeepsBytesWhenATrashWindowIsSet(t *testing.T) {
	service, ctx := uploads(t, upload.Rules{}, time.Hour)
	body := pngBytes(t, 2, 2)

	file, _ := service.Store(ctx, bytes.NewReader(body), "a.png", "image/png", int64(len(body)))
	ref := file.Ref
	if err := service.Commit(ctx, &ref); err != nil {
		t.Fatal(err)
	}
	if err := service.Delete(ctx, ref); err != nil {
		t.Fatal(err)
	}

	if service.Storage().Exists(ctx, string(ref)) {
		t.Error("the live copy should be gone")
	}
	trashed, err := service.Storage().List(ctx, upload.TrashPrefix)
	if err != nil {
		t.Fatal(err)
	}
	if len(trashed) != 1 {
		t.Errorf("the bytes should be recoverable from trash, got %+v", trashed)
	}
}

func TestSweepClearsAbandonedStagedFiles(t *testing.T) {
	service := upload.New(upload.Options{
		Storage:  storage.NewFileSystem(filepath.Join(t.TempDir(), "media")),
		Rules:    upload.Rules{Allowed: []string{"image/png"}},
		StageTTL: time.Nanosecond,
	})
	ctx := context.Background()
	body := pngBytes(t, 2, 2)

	if _, err := service.Store(ctx, bytes.NewReader(body), "a.png", "image/png", int64(len(body))); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)

	staged, _, err := service.Sweep(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if staged != 1 {
		t.Errorf("swept %d staged files, want 1", staged)
	}

	left, err := service.Storage().List(ctx, upload.StagedPrefix)
	if err != nil {
		t.Fatal(err)
	}
	if len(left) != 0 {
		t.Errorf("orphans should be gone, got %+v", left)
	}
}

func TestCommittedFilesSurviveTheSweep(t *testing.T) {
	service := upload.New(upload.Options{
		Storage:  storage.NewFileSystem(filepath.Join(t.TempDir(), "media")),
		Rules:    upload.Rules{Allowed: []string{"image/png"}},
		StageTTL: time.Nanosecond,
	})
	ctx := context.Background()
	body := pngBytes(t, 2, 2)

	file, _ := service.Store(ctx, bytes.NewReader(body), "a.png", "image/png", int64(len(body)))
	ref := file.Ref
	if err := service.Commit(ctx, &ref); err != nil {
		t.Fatal(err)
	}
	time.Sleep(2 * time.Millisecond)

	if _, _, err := service.Sweep(ctx); err != nil {
		t.Fatal(err)
	}
	if !service.Storage().Exists(ctx, string(ref)) {
		t.Error("a committed file must never be swept")
	}
}

func multipartRequest(t *testing.T, field, filename string, body []byte, declared string) *http.Request {
	t.Helper()
	buffer := &bytes.Buffer{}
	writer := multipart.NewWriter(buffer)

	header := make(map[string][]string)
	header["Content-Disposition"] = []string{`form-data; name="` + field + `"; filename="` + filename + `"`}
	if declared != "" {
		header["Content-Type"] = []string{declared}
	}
	part, err := writer.CreatePart(header)
	if err != nil {
		t.Fatal(err)
	}
	part.Write(body)
	writer.Close()

	req := httptest.NewRequest(http.MethodPost, "/upload", buffer)
	req.Header.Set("Content-Type", writer.FormDataContentType())
	return req
}

func TestAcceptReadsAMultipartRequest(t *testing.T) {
	service, _ := uploads(t, upload.Rules{}, 0)
	body := pngBytes(t, 3, 3)

	file, err := service.Accept(multipartRequest(t, "photo", "snap.png", body, "image/png"), "photo")
	if err != nil {
		t.Fatal(err)
	}
	if file.Type != "image/png" || !file.Ref.Staged() {
		t.Errorf("file = %+v", file)
	}

	if _, err := service.Accept(multipartRequest(t, "photo", "snap.png", body, "image/png"), "other"); !errors.Is(err, upload.ErrNoFile) {
		t.Errorf("a missing field should report ErrNoFile, got %v", err)
	}
}

func TestRefRoundTripsThroughTheDatabase(t *testing.T) {
	ref := upload.Ref("ab/cd/abcdef.png")

	value, err := ref.Value()
	if err != nil {
		t.Fatal(err)
	}
	if value != "ab/cd/abcdef.png" {
		t.Errorf("Value = %v", value)
	}

	var scanned upload.Ref
	if err := scanned.Scan([]byte("ab/cd/abcdef.png")); err != nil {
		t.Fatal(err)
	}
	if scanned != ref {
		t.Errorf("Scan = %q", scanned)
	}
	if err := scanned.Scan(nil); err != nil || !scanned.Empty() {
		t.Errorf("a null column should read as empty, got %q / %v", scanned, err)
	}
	if scanned.Scan(42) == nil {
		t.Error("an unexpected type should be an error")
	}
	if ref.Digest() != "abcdef" || ref.Extension() != ".png" {
		t.Errorf("digest/extension = %q/%q", ref.Digest(), ref.Extension())
	}
}

func servedUploads(t *testing.T) (*upload.Service, http.Handler, upload.Ref) {
	t.Helper()
	service, ctx := uploads(t, upload.Rules{Allowed: []string{"image/png", "text/plain"}}, 0)

	body := pngBytes(t, 2, 2)
	file, err := service.Store(ctx, bytes.NewReader(body), "a.png", "image/png", int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	ref := file.Ref
	if err := service.Commit(ctx, &ref); err != nil {
		t.Fatal(err)
	}
	return service, service.Handler(upload.HandlerOptions{Prefix: "/media/"}), ref
}

func TestServedFilesCarrySafeHeaders(t *testing.T) {
	service, handler, ref := servedUploads(t)

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/"+string(ref), nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d", rec.Code)
	}
	if got := rec.Header().Get("X-Content-Type-Options"); got != "nosniff" {
		t.Errorf("nosniff = %q", got)
	}
	if got := rec.Header().Get("Content-Type"); got != "image/png" {
		t.Errorf("Content-Type = %q", got)
	}
	if got := rec.Header().Get("Content-Disposition"); got != "" {
		t.Errorf("an image may render inline, got %q", got)
	}
	if !strings.Contains(rec.Header().Get("Cache-Control"), "immutable") {
		t.Errorf("content-addressed files should cache hard, got %q", rec.Header().Get("Cache-Control"))
	}

	ctx := context.Background()
	text, err := service.Store(ctx, strings.NewReader("plain words here"), "notes.txt", "", 16)
	if err != nil {
		t.Fatal(err)
	}
	plain := text.Ref
	if err := service.Commit(ctx, &plain); err != nil {
		t.Fatal(err)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/"+string(plain), nil))
	if got := rec.Header().Get("Content-Disposition"); got != "attachment" {
		t.Errorf("non-image content must download, not render: %q", got)
	}
}

func TestStagedAndTrashedFilesAreNeverServed(t *testing.T) {
	service, handler, _ := servedUploads(t)
	ctx := context.Background()

	body := pngBytes(t, 3, 3)
	staged, err := service.Store(ctx, bytes.NewReader(body), "b.png", "image/png", int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/"+string(staged.Ref), nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("a staged file should not be reachable: %d", rec.Code)
	}

	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/trash/ab/cd/whatever.png", nil))
	if rec.Code != http.StatusNotFound {
		t.Errorf("trash should not be reachable: %d", rec.Code)
	}
}

func TestPrivateFilesNeedASignature(t *testing.T) {
	service, _, ref := servedUploads(t)
	const secret = "a-test-secret-key-long-enough-for-hmac"

	handler := service.Handler(upload.HandlerOptions{Prefix: "/media/", Private: true, Secret: secret})

	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/media/"+string(ref), nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("an unsigned request should be refused: %d", rec.Code)
	}

	signed := service.SignedURL("/media/", secret, ref, time.Minute)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, signed, nil))
	if rec.Code != http.StatusOK {
		t.Errorf("a signed request should pass: %d", rec.Code)
	}

	expired := service.SignedURL("/media/", secret, ref, -time.Minute)
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, expired, nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("an expired signature should be refused: %d", rec.Code)
	}

	tampered := strings.Replace(signed, string(ref), string(ref), 1) + "x"
	rec = httptest.NewRecorder()
	handler.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, tampered, nil))
	if rec.Code != http.StatusForbidden {
		t.Errorf("a tampered signature should be refused: %d", rec.Code)
	}
}

func TestRangeRequestsWork(t *testing.T) {
	_, handler, ref := servedUploads(t)

	req := httptest.NewRequest(http.MethodGet, "/media/"+string(ref), nil)
	req.Header.Set("Range", "bytes=0-9")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	if rec.Code != http.StatusPartialContent {
		t.Errorf("status = %d, want 206 so media can seek", rec.Code)
	}
	if rec.Body.Len() != 10 {
		t.Errorf("returned %d bytes, want 10", rec.Body.Len())
	}
}

func TestAGenericDeclaredTypeIsNotTreatedAsALie(t *testing.T) {
	service, ctx := uploads(t, upload.Rules{Allowed: []string{"image/png"}}, 0)
	body := pngBytes(t, 2, 2)

	for _, declared := range []string{"", "application/octet-stream"} {
		if _, err := service.Store(ctx, bytes.NewReader(body), "a.png", declared, int64(len(body))); err != nil {
			t.Errorf("declared %q should be accepted, got %v", declared, err)
		}
	}
}

func TestUploadPathsCannotEscapeTheMediaRoot(t *testing.T) {
	for input, want := range map[string]string{
		"products/photos":   "products/photos",
		"/products/photos/": "products/photos",
		"../../etc":         "etc",
		"a/../../b":         "a/b",
		"":                  "",
		"   ":               "",
		`windows\path`:      "windows/path",
	} {
		if got := upload.CleanPath(input); got != want {
			t.Errorf("CleanPath(%q) = %q, want %q", input, got, want)
		}
	}
}

func TestStoreToPlacesFilesUnderThePath(t *testing.T) {
	service, ctx := uploads(t, upload.Rules{Allowed: []string{"image/png"}}, 0)
	body := pngBytes(t, 2, 2)

	file, err := service.StoreTo(ctx, bytes.NewReader(body), "a.png", "image/png", int64(len(body)), "posters/images")
	if err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(file.Ref), "staged/posters/images/") {
		t.Errorf("ref = %q", file.Ref)
	}

	ref := file.Ref
	if err := service.Commit(ctx, &ref); err != nil {
		t.Fatal(err)
	}
	if !strings.HasPrefix(string(ref), "posters/images/") {
		t.Errorf("committed ref = %q, the path should survive the promotion", ref)
	}
}
