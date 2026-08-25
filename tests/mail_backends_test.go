package tests

import (
	"context"
	"errors"
	"fmt"
	netmail "net/mail"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"

	"github.com/farhapartex/coyote/lib/mail"
)

type recordingSender struct {
	mu       sync.Mutex
	sent     []mail.Message
	raw      [][]byte
	failWith error
}

func (r *recordingSender) Send(ctx context.Context, message mail.Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if r.failWith != nil {
		return r.failWith
	}

	encoded, err := mail.Encode(message)
	if err != nil {
		return err
	}

	r.mu.Lock()
	defer r.mu.Unlock()

	r.sent = append(r.sent, message)
	r.raw = append(r.raw, encoded)
	return nil
}

var _ mail.Sender = (*recordingSender)(nil)

func TestAnApplicationCanDefineItsOwnBackend(t *testing.T) {
	var sender mail.Sender = &recordingSender{}

	if err := sender.Send(context.Background(), validMessage()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	backend := sender.(*recordingSender)
	if len(backend.sent) != 1 {
		t.Fatalf("the backend recorded %d messages", len(backend.sent))
	}
	if !strings.Contains(string(backend.raw[0]), "Subject: Your order") {
		t.Error("a custom backend should be able to encode with the exported API")
	}

	failing := &recordingSender{failWith: errors.New("the relay is down")}
	if err := failing.Send(context.Background(), validMessage()); err == nil {
		t.Error("a backend must be able to report an error")
	}
}

func TestACustomBackendCanDecorateAnother(t *testing.T) {
	inner := mail.NewMemory()
	calls := 0

	var sender mail.Sender = senderFunc(func(ctx context.Context, message mail.Message) error {
		calls++
		return inner.Send(ctx, message.WithHeader("X-Traced", "yes"))
	})

	if err := sender.Send(context.Background(), validMessage()); err != nil {
		t.Fatalf("Send: %v", err)
	}

	last, ok := inner.Last()
	if !ok || last.Headers["X-Traced"] != "yes" {
		t.Errorf("the decorator did not reach the inner backend: %+v", last)
	}
	if calls != 1 {
		t.Errorf("calls = %d", calls)
	}
}

type senderFunc func(context.Context, mail.Message) error

func (f senderFunc) Send(ctx context.Context, message mail.Message) error {
	return f(ctx, message)
}

func TestConsoleBackendWritesAParseableMessage(t *testing.T) {
	var out strings.Builder
	sender := mail.NewConsole(&out)

	for _, subject := range []string{"First", "Second"} {
		message := validMessage()
		message.Subject = subject
		if err := sender.Send(context.Background(), message); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}

	written := out.String()
	if strings.Count(written, "Subject: First") != 1 || strings.Count(written, "Subject: Second") != 1 {
		t.Errorf("both messages should appear once:\n%s", written)
	}
	if strings.Count(written, "coyote/mail") != 2 {
		t.Error("consecutive messages should be separated legibly")
	}

	first := written[strings.Index(written, "From:"):]
	if _, err := netmail.ReadMessage(strings.NewReader(first)); err != nil {
		t.Errorf("the console output does not parse: %v", err)
	}
}

func TestFileBackendWritesOneFilePerSend(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "not", "yet", "there")
	sender := mail.NewFile(dir)

	if sender.Dir() != dir {
		t.Errorf("Dir = %q, want %q", sender.Dir(), dir)
	}

	for i := 0; i < 3; i++ {
		message := validMessage()
		message.Subject = fmt.Sprintf("Order %d", i)
		if err := sender.Send(context.Background(), message); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("the directory was not created: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("the directory holds %d files, want 3", len(entries))
	}

	for _, entry := range entries {
		if filepath.Ext(entry.Name()) != ".eml" {
			t.Errorf("%q is not an .eml file", entry.Name())
		}

		info, err := entry.Info()
		if err != nil {
			t.Fatalf("Info: %v", err)
		}
		if perm := info.Mode().Perm(); perm != 0o600 {
			t.Errorf("%s has mode %v, want 0600", entry.Name(), perm)
		}

		content, err := os.ReadFile(filepath.Join(dir, entry.Name()))
		if err != nil {
			t.Fatalf("ReadFile: %v", err)
		}
		if _, err := netmail.ReadMessage(strings.NewReader(string(content))); err != nil {
			t.Errorf("%s does not parse: %v", entry.Name(), err)
		}
	}
}

func TestMemoryBackendRecordsInOrder(t *testing.T) {
	sender := mail.NewMemory()

	if _, ok := sender.Last(); ok {
		t.Error("an empty backend should report no last message")
	}
	if sender.Count() != 0 {
		t.Errorf("Count = %d", sender.Count())
	}

	for _, subject := range []string{"One", "Two", "Three"} {
		message := validMessage()
		message.Subject = subject
		if err := sender.Send(context.Background(), message); err != nil {
			t.Fatalf("Send: %v", err)
		}
	}

	if sender.Count() != 3 {
		t.Errorf("Count = %d, want 3", sender.Count())
	}

	sent := sender.Sent()
	for i, want := range []string{"One", "Two", "Three"} {
		if sent[i].Subject != want {
			t.Errorf("position %d = %q, want %q", i, sent[i].Subject, want)
		}
	}

	last, ok := sender.Last()
	if !ok || last.Subject != "Three" {
		t.Errorf("Last = %q (%v)", last.Subject, ok)
	}
	if raw := sender.Raw(); len(raw) != 3 || !strings.Contains(string(raw[2]), "Subject: Three") {
		t.Error("Raw should hold the encoded bytes of every send")
	}

	sender.Clear()
	if sender.Count() != 0 || len(sender.Sent()) != 0 || len(sender.Raw()) != 0 {
		t.Error("Clear should empty the record")
	}
	if _, ok := sender.Last(); ok {
		t.Error("Last should report nothing after Clear")
	}
}

func TestMemoryBackendReturnsACopy(t *testing.T) {
	sender := mail.NewMemory()

	message := validMessage()
	message.To = []string{"ada@example.test"}
	message = message.WithHeader("X-Campaign", "spring").
		Attach(mail.NewAttachment("a.txt", "text/plain", []byte("hello")))

	if err := sender.Send(context.Background(), message); err != nil {
		t.Fatalf("Send: %v", err)
	}

	message.Subject = "changed after sending"
	message.To[0] = "attacker@evil.test"
	message.Headers["X-Campaign"] = "tampered"
	message.Attachments[0].Content[0] = 'X'

	recorded := sender.Sent()[0]
	if recorded.Subject != "Your order" {
		t.Errorf("Subject = %q, the record followed the caller", recorded.Subject)
	}
	if recorded.To[0] != "ada@example.test" {
		t.Errorf("To = %v, the recipient slice is shared", recorded.To)
	}
	if recorded.Headers["X-Campaign"] != "spring" {
		t.Errorf("Headers = %v, the map is shared", recorded.Headers)
	}
	if string(recorded.Attachments[0].Content) != "hello" {
		t.Errorf("attachment = %q, the content is shared", recorded.Attachments[0].Content)
	}

	recorded.Subject = "mutating what Sent returned"
	if again := sender.Sent()[0]; again.Subject != "Your order" {
		t.Errorf("Subject = %q, Sent handed out the record itself", again.Subject)
	}
}

func TestEveryBackendRejectsAnInvalidMessageBeforeWriting(t *testing.T) {
	dir := t.TempDir()
	var console strings.Builder

	memory := mail.NewMemory()
	backends := map[string]mail.Sender{
		"console": mail.NewConsole(&console),
		"file":    mail.NewFile(dir),
		"memory":  memory,
	}

	injected := validMessage()
	injected.Subject = "Hi\r\nBcc: attacker@evil.test"

	for name, sender := range backends {
		if err := sender.Send(context.Background(), mail.Message{}); !errors.Is(err, mail.ErrNoSender) {
			t.Errorf("%s accepted an empty message: %v", name, err)
		}
		if err := sender.Send(context.Background(), injected); !errors.Is(err, mail.ErrHeaderInjection) {
			t.Errorf("%s accepted an injected header: %v", name, err)
		}
	}

	if console.Len() != 0 {
		t.Errorf("the console wrote something for a rejected message:\n%s", console.String())
	}
	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Errorf("the file backend left %d files behind", len(entries))
	}
	if memory.Count() != 0 {
		t.Errorf("the memory backend recorded %d rejected messages", memory.Count())
	}
}

func TestEveryBackendHonoursACancelledContext(t *testing.T) {
	dir := t.TempDir()
	var console strings.Builder

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	for name, sender := range map[string]mail.Sender{
		"console": mail.NewConsole(&console),
		"file":    mail.NewFile(dir),
		"memory":  mail.NewMemory(),
	} {
		if err := sender.Send(ctx, validMessage()); !errors.Is(err, context.Canceled) {
			t.Errorf("%s ignored a cancelled context: %v", name, err)
		}
	}

	if entries, _ := os.ReadDir(dir); len(entries) != 0 {
		t.Error("the file backend wrote despite cancellation")
	}
	if console.Len() != 0 {
		t.Error("the console wrote despite cancellation")
	}
}

func TestBackendsAreSafeUnderConcurrentSends(t *testing.T) {
	const senders = 50

	dir := t.TempDir()
	var console strings.Builder

	memory := mail.NewMemory()
	fileSender := mail.NewFile(dir)
	consoleSender := mail.NewConsole(&console)

	var wait sync.WaitGroup
	errs := make(chan error, senders*3)

	for i := 0; i < senders; i++ {
		message := validMessage()
		message.Subject = fmt.Sprintf("Order %d", i)

		for _, sender := range []mail.Sender{memory, fileSender, consoleSender} {
			wait.Add(1)
			go func(sender mail.Sender) {
				defer wait.Done()
				if err := sender.Send(context.Background(), message); err != nil {
					errs <- err
				}
			}(sender)
		}
	}

	wait.Wait()
	close(errs)
	for err := range errs {
		t.Errorf("a concurrent send failed: %v", err)
	}

	if memory.Count() != senders {
		t.Errorf("the memory backend holds %d messages, want %d", memory.Count(), senders)
	}

	entries, err := os.ReadDir(dir)
	if err != nil {
		t.Fatalf("ReadDir: %v", err)
	}
	if len(entries) != senders {
		t.Errorf("the file backend wrote %d files, want %d distinct ones", len(entries), senders)
	}

	names := map[string]bool{}
	for _, entry := range entries {
		if names[entry.Name()] {
			t.Errorf("%q was written twice", entry.Name())
		}
		names[entry.Name()] = true
	}

	if got := strings.Count(console.String(), "MIME-Version: 1.0"); got != senders {
		t.Errorf("the console wrote %d messages, want %d", got, senders)
	}

	subjects := map[string]bool{}
	for _, message := range memory.Sent() {
		subjects[message.Subject] = true
	}
	if len(subjects) != senders {
		t.Errorf("the memory backend kept %d distinct subjects, want %d", len(subjects), senders)
	}
}
