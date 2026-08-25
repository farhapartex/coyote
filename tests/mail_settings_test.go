package tests

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/farhapartex/coyote/core/settings"
	"github.com/farhapartex/coyote/lib/mail"
)

func emailSettings(t *testing.T, email settings.Email) (settings.Settings, error) {
	t.Helper()

	return settings.New(prodSettings(func(s *settings.Settings) {
		s.BaseDir = t.TempDir()
		s.Email = email
	})...)
}

func emailProblems(t *testing.T, email settings.Email) []string {
	t.Helper()

	_, err := emailSettings(t, email)
	if err == nil {
		return nil
	}

	var about []string
	for _, problem := range problemsOf(t, err) {
		if strings.Contains(problem, "Email") {
			about = append(about, problem)
		}
	}
	return about
}

func mustBuildEmail(t *testing.T, email settings.Email) settings.Settings {
	t.Helper()

	s, err := emailSettings(t, email)
	if err != nil {
		t.Fatalf("that configuration should be accepted: %v", err)
	}
	return s
}

func TestEmailIsOffByDefault(t *testing.T) {
	s, err := settings.New(prodSettings()...)
	if err != nil {
		t.Fatalf("a project that never sends email must configure nothing: %v", err)
	}

	if s.Email.Enabled() {
		t.Error("Enabled should be false by default")
	}
	if s.Email.Backend != "" {
		t.Errorf("Backend = %q, want empty", s.Email.Backend)
	}
	if got := s.Email.Location(); got != "not configured" {
		t.Errorf("Location = %q", got)
	}

	if _, err := s.Email.Open(); !errors.Is(err, mail.ErrNotConfigured) {
		t.Errorf("Open = %v, want ErrNotConfigured", err)
	}
}

func TestEmailOpenReturnsTheRightBackend(t *testing.T) {
	dir := t.TempDir()

	for name, tc := range map[string]struct {
		email settings.Email
		want  string
	}{
		"smtp": {
			email: settings.Email{Backend: settings.EmailToSMTP, Host: "smtp.example.test"},
			want:  "*mail.SMTPSender",
		},
		"console": {
			email: settings.Email{Backend: settings.EmailToConsole},
			want:  "*mail.ConsoleSender",
		},
		"file": {
			email: settings.Email{Backend: settings.EmailToFile, Dir: dir},
			want:  "*mail.FileSender",
		},
		"memory": {
			email: settings.Email{Backend: settings.EmailToMemory},
			want:  "*mail.MemorySender",
		},
	} {
		t.Run(name, func(t *testing.T) {
			sender, err := tc.email.Open()
			if err != nil {
				t.Fatalf("Open: %v", err)
			}
			if got := fmt.Sprintf("%T", sender); got != tc.want {
				t.Errorf("Open gave %s, want %s", got, tc.want)
			}
		})
	}
}

func TestEmailOpenReturnsASuppliedSenderUntouched(t *testing.T) {
	own := mail.NewMemory()
	email := settings.Email{From: "hello@shop.test", Sender: own}

	if !email.Enabled() {
		t.Error("a supplied Sender should count as configured")
	}

	sender, err := email.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}
	if sender != mail.Sender(own) {
		t.Error("Open should hand back exactly the supplied Sender")
	}

	if err := sender.Send(context.Background(), validMessage()); err != nil {
		t.Fatalf("Send: %v", err)
	}
	if own.Count() != 1 {
		t.Errorf("the supplied backend recorded %d messages", own.Count())
	}
	if !strings.Contains(email.Location(), "MemorySender") {
		t.Errorf("Location = %q, it should name the supplied type", email.Location())
	}
}

func TestEmailRefusesASenderAlongsideABackend(t *testing.T) {
	problems := emailProblems(t, settings.Email{Backend: settings.EmailToMemory, Sender: mail.NewMemory()})

	if len(problems) == 0 {
		t.Fatal("setting both Sender and Backend should be refused")
	}
	if !strings.Contains(problems[0], "choose one") {
		t.Errorf("problem = %q", problems[0])
	}
}

func TestEmailValidatesASuppliedSendersConfiguration(t *testing.T) {
	problems := emailProblems(t, settings.Email{Sender: mail.NewMemory(), From: "not an address"})

	if len(problems) != 1 {
		t.Fatalf("problems = %v, want one about From", problems)
	}
	if !strings.Contains(problems[0], "Email.From") {
		t.Errorf("problem = %q", problems[0])
	}
}

func TestEmailValidation(t *testing.T) {
	for name, tc := range map[string]struct {
		email settings.Email
		want  string
	}{
		"unknown backend": {
			email: settings.Email{Backend: "carrier-pigeon"},
			want:  "is not supported",
		},
		"smtp without a host": {
			email: settings.Email{Backend: settings.EmailToSMTP},
			want:  "Email.Host is empty",
		},
		"impossible port": {
			email: settings.Email{Backend: settings.EmailToSMTP, Host: "smtp.example.test", Port: 70000},
			want:  "is not a port",
		},
		"unknown tls mode": {
			email: settings.Email{Backend: settings.EmailToSMTP, Host: "smtp.example.test", TLS: "sometimes"},
			want:  "Email.TLS",
		},
		"credentials without encryption": {
			email: settings.Email{
				Backend: settings.EmailToSMTP, Host: "smtp.example.test",
				TLS: mail.TLSNone, Username: "shop", Password: "s3cret",
			},
			want: "in clear",
		},
		"unparseable from": {
			email: settings.Email{Backend: settings.EmailToConsole, From: "@@@"},
			want:  "is not a usable address",
		},
		"negative timeout": {
			email: settings.Email{Backend: settings.EmailToConsole, Timeout: -time.Second},
			want:  "cannot be negative",
		},
		"host on console": {
			email: settings.Email{Backend: settings.EmailToConsole, Host: "smtp.example.test"},
			want:  "does not connect to a server",
		},
		"credentials on memory": {
			email: settings.Email{Backend: settings.EmailToMemory, Username: "shop"},
			want:  "does not authenticate",
		},
		"tls on file": {
			email: settings.Email{Backend: settings.EmailToFile, Dir: "mail", TLS: mail.TLSStartTLS},
			want:  "does not use TLS",
		},
		"fields without a backend": {
			email: settings.Email{Host: "smtp.example.test"},
			want:  "Email.Backend is empty but other Email fields are set",
		},
	} {
		t.Run(name, func(t *testing.T) {
			problems := emailProblems(t, tc.email)
			if len(problems) == 0 {
				t.Fatalf("%+v should be refused", tc.email)
			}

			joined := strings.Join(problems, " | ")
			if !strings.Contains(joined, tc.want) {
				t.Errorf("problems = %s, want one mentioning %q", joined, tc.want)
			}
		})
	}
}

func TestEmailFileBackendAcceptsADefaultDirectory(t *testing.T) {
	s := mustBuildEmail(t, settings.Email{Backend: settings.EmailToFile})

	if !filepath.IsAbs(s.Email.Dir) {
		t.Errorf("Dir = %q, it should be resolved against BaseDir", s.Email.Dir)
	}
	if filepath.Base(s.Email.Dir) != "mail" {
		t.Errorf("Dir = %q, want it to end in mail", s.Email.Dir)
	}
}

func TestEmailDirResolvesAgainstBaseDir(t *testing.T) {
	s := mustBuildEmail(t, settings.Email{Backend: settings.EmailToFile, Dir: "outbox"})

	if want := filepath.Join(s.BaseDir, "outbox"); s.Email.Dir != want {
		t.Errorf("Dir = %q, want %q", s.Email.Dir, want)
	}

	absolute := filepath.Join(t.TempDir(), "elsewhere")
	s = mustBuildEmail(t, settings.Email{Backend: settings.EmailToFile, Dir: absolute})
	if s.Email.Dir != absolute {
		t.Errorf("Dir = %q, an absolute directory should be left alone", s.Email.Dir)
	}
}

func TestEmailPortDefaultsFollowTheTLSMode(t *testing.T) {
	for mode, want := range map[mail.TLSMode]int{
		mail.TLSStartTLS: 587,
		mail.TLSImplicit: 465,
		mail.TLSNone:     25,
		"":               587,
	} {
		s := mustBuildEmail(t, settings.Email{
			Backend: settings.EmailToSMTP, Host: "smtp.example.test", TLS: mode,
		})
		if s.Email.Port != want {
			t.Errorf("mode %q gave port %d, want %d", mode, s.Email.Port, want)
		}
	}

	s := mustBuildEmail(t, settings.Email{
		Backend: settings.EmailToSMTP, Host: "smtp.example.test", Port: 2525,
	})
	if s.Email.Port != 2525 {
		t.Errorf("Port = %d, an explicit port should win", s.Email.Port)
	}
	if !strings.HasSuffix(s.Email.Location(), ":2525") {
		t.Errorf("Location = %q", s.Email.Location())
	}
}

func TestEmailTimeoutDefaults(t *testing.T) {
	s := mustBuildEmail(t, settings.Email{Backend: settings.EmailToConsole})
	if s.Email.Timeout != 10*time.Second {
		t.Errorf("Timeout = %v, want 10s", s.Email.Timeout)
	}
}

func TestEmailRedactedHidesThePassword(t *testing.T) {
	email := settings.Email{
		Backend:  settings.EmailToSMTP,
		Host:     "smtp.example.test",
		TLS:      mail.TLSStartTLS,
		Username: "shop",
		Password: "s3cret-do-not-print",
	}

	redacted := email.Redacted()
	if strings.Contains(redacted.Password, "s3cret") {
		t.Errorf("Redacted kept the password: %q", redacted.Password)
	}
	if redacted.Password == "" {
		t.Error("Redacted should mask the password, not drop it")
	}
	if email.Password != "s3cret-do-not-print" {
		t.Error("Redacted mutated the original")
	}
	if redacted.Username != "shop" || redacted.Host != "smtp.example.test" {
		t.Errorf("Redacted changed more than the password: %+v", redacted)
	}

	empty := settings.Email{Backend: settings.EmailToConsole}.Redacted()
	if empty.Password != "" {
		t.Errorf("an absent password should stay absent, got %q", empty.Password)
	}
}

func TestEmailPasswordNeverReachesAConfigurationError(t *testing.T) {
	_, err := emailSettings(t, settings.Email{
		Backend:  settings.EmailToSMTP,
		TLS:      mail.TLSNone,
		Username: "shop",
		Password: "s3cret-do-not-print",
	})

	if err == nil {
		t.Fatal("that configuration should be refused")
	}
	if strings.Contains(err.Error(), "s3cret-do-not-print") {
		t.Errorf("the password leaked into the error: %v", err)
	}
}

func TestEmailSMTPBackendIsUsableFromSettings(t *testing.T) {
	server := newFakeSMTP(t)

	s := mustBuildEmail(t, settings.Email{
		Backend: settings.EmailToSMTP,
		Host:    server.Host(),
		Port:    server.Port(),
		TLS:     mail.TLSNone,
		From:    "shop@example.test",
	})

	sender, err := s.Email.Open()
	if err != nil {
		t.Fatalf("Open: %v", err)
	}

	message := mail.Message{
		To:      []string{"ada@example.test"},
		Subject: "From settings",
		Text:    "Configured, opened, sent.",
	}
	if err := sender.Send(context.Background(), message); err != nil {
		t.Fatalf("Send: %v", err)
	}

	record, ok := server.Last()
	if !ok {
		t.Fatal("the server saw nothing")
	}
	if !strings.Contains(record.data, "Subject: From settings") {
		t.Errorf("the message did not arrive:\n%s", record.data)
	}
	if !strings.Contains(commandsOf(record), "MAIL FROM:<shop@example.test>") {
		t.Errorf("the configured From was not used: %s", commandsOf(record))
	}
}
