package tests

import (
	"bufio"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"crypto/tls"
	"crypto/x509"
	"crypto/x509/pkix"
	"encoding/base64"
	"encoding/pem"
	"fmt"
	"math/big"
	"net"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type smtpConversation struct {
	commands      []string
	data          string
	upgradedToTLS bool
	authenticated bool
	credentials   []string
}

type fakeSMTP struct {
	t           *testing.T
	listener    net.Listener
	tlsConfig   *tls.Config
	extensions  []string
	replies     map[string]string
	silent      bool
	done        chan struct{}
	implicitTLS bool

	mu            sync.Mutex
	conversations []smtpConversation
	closed        bool
	wait          sync.WaitGroup
}

func newFakeSMTP(t *testing.T, extensions ...string) *fakeSMTP {
	return startFakeSMTP(t, false, extensions...)
}

func newFakeSMTPWithImplicitTLS(t *testing.T, extensions ...string) *fakeSMTP {
	return startFakeSMTP(t, true, extensions...)
}

func startFakeSMTP(t *testing.T, implicitTLS bool, extensions ...string) *fakeSMTP {
	t.Helper()

	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}

	server := &fakeSMTP{
		t:           t,
		listener:    listener,
		tlsConfig:   selfSignedConfig(t),
		extensions:  extensions,
		replies:     map[string]string{},
		done:        make(chan struct{}),
		implicitTLS: implicitTLS,
	}

	server.wait.Add(1)
	go server.accept()

	t.Cleanup(server.Close)
	return server
}

func (f *fakeSMTP) Host() string {
	host, _, _ := net.SplitHostPort(f.listener.Addr().String())
	return host
}

func (f *fakeSMTP) Port() int {
	_, port, _ := net.SplitHostPort(f.listener.Addr().String())
	number, _ := strconv.Atoi(port)
	return number
}

func (f *fakeSMTP) ClientTLS() *tls.Config {
	return &tls.Config{InsecureSkipVerify: true, ServerName: f.Host()}
}

func (f *fakeSMTP) Reply(verb, reply string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.replies[strings.ToUpper(verb)] = reply
}

func (f *fakeSMTP) Silence() {
	f.mu.Lock()
	defer f.mu.Unlock()

	f.silent = true
}

func (f *fakeSMTP) Conversations() []smtpConversation {
	f.mu.Lock()
	defer f.mu.Unlock()

	out := make([]smtpConversation, len(f.conversations))
	copy(out, f.conversations)
	return out
}

func (f *fakeSMTP) Last() (smtpConversation, bool) {
	all := f.Conversations()
	if len(all) == 0 {
		return smtpConversation{}, false
	}
	return all[len(all)-1], true
}

func (f *fakeSMTP) Close() {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return
	}
	f.closed = true
	f.mu.Unlock()

	close(f.done)
	f.listener.Close()
	f.wait.Wait()
}

func (f *fakeSMTP) accept() {
	defer f.wait.Done()

	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}

		f.wait.Add(1)
		go func() {
			defer f.wait.Done()
			defer conn.Close()
			f.serve(conn)
		}()
	}
}

func (f *fakeSMTP) settings() (bool, map[string]string, []string) {
	f.mu.Lock()
	defer f.mu.Unlock()

	replies := make(map[string]string, len(f.replies))
	for verb, reply := range f.replies {
		replies[verb] = reply
	}
	return f.silent, replies, append([]string(nil), f.extensions...)
}

func (f *fakeSMTP) serve(conn net.Conn) {
	silent, replies, extensions := f.settings()
	if silent {
		select {
		case <-f.done:
		case <-time.After(30 * time.Second):
		}
		return
	}

	conn.SetDeadline(time.Now().Add(10 * time.Second))

	record := smtpConversation{}
	if f.implicitTLS {
		secure := tls.Server(conn, f.tlsConfig)
		if err := secure.Handshake(); err != nil {
			return
		}
		record.upgradedToTLS = true
		conn = secure
	}

	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	say := func(format string, args ...any) bool {
		fmt.Fprintf(writer, format+"\r\n", args...)
		return writer.Flush() == nil
	}

	if !say("220 fake.test ESMTP coyote") {
		return
	}

	for {
		line, err := reader.ReadString('\n')
		if err != nil {
			f.store(record)
			return
		}

		line = strings.TrimRight(line, "\r\n")
		record.commands = append(record.commands, line)
		verb, rest, _ := strings.Cut(line, " ")
		verb = strings.ToUpper(verb)

		if override, ok := replies[verb]; ok && verb != "DATA" {
			if !say("%s", override) {
				return
			}
			continue
		}

		switch verb {
		case "EHLO":
			say("250-fake.test greets you")
			for i, extension := range extensions {
				if i == len(extensions)-1 {
					say("250 %s", extension)
				} else {
					say("250-%s", extension)
				}
			}
			if len(extensions) == 0 {
				say("250 HELP")
			}

		case "HELO":
			say("250 fake.test")

		case "STARTTLS":
			if !say("220 ready to start TLS") {
				return
			}
			secure := tls.Server(conn, f.tlsConfig)
			if err := secure.Handshake(); err != nil {
				f.store(record)
				return
			}
			record.upgradedToTLS = true
			conn = secure
			reader = bufio.NewReader(secure)
			writer = bufio.NewWriter(secure)
			say = func(format string, args ...any) bool {
				fmt.Fprintf(writer, format+"\r\n", args...)
				return writer.Flush() == nil
			}

		case "AUTH":
			mechanism, initial, _ := strings.Cut(rest, " ")
			switch strings.ToUpper(mechanism) {
			case "PLAIN":
				record.credentials = append(record.credentials, initial)
				record.authenticated = true
				say("235 2.7.0 authenticated")
			case "LOGIN":
				say("334 %s", base64.StdEncoding.EncodeToString([]byte("Username:")))
				username, err := reader.ReadString('\n')
				if err != nil {
					f.store(record)
					return
				}
				record.credentials = append(record.credentials, strings.TrimSpace(username))

				say("334 %s", base64.StdEncoding.EncodeToString([]byte("Password:")))
				password, err := reader.ReadString('\n')
				if err != nil {
					f.store(record)
					return
				}
				record.credentials = append(record.credentials, strings.TrimSpace(password))
				record.authenticated = true
				say("235 2.7.0 authenticated")
			default:
				say("504 unrecognised authentication type")
			}

		case "MAIL", "RCPT", "RSET", "NOOP":
			say("250 2.1.0 ok")

		case "DATA":
			if override, ok := replies["DATA"]; ok {
				say("%s", override)
				continue
			}
			if !say("354 end with <CRLF>.<CRLF>") {
				return
			}

			var body strings.Builder
			for {
				dataLine, err := reader.ReadString('\n')
				if err != nil {
					f.store(record)
					return
				}
				if dataLine == ".\r\n" || dataLine == ".\n" {
					break
				}
				body.WriteString(dataLine)
			}
			record.data = body.String()
			say("250 2.0.0 queued")

		case "QUIT":
			say("221 2.0.0 bye")
			f.store(record)
			return

		default:
			say("500 5.5.1 unrecognised command")
		}
	}
}

func (f *fakeSMTP) store(record smtpConversation) {
	if len(record.commands) == 0 {
		return
	}

	f.mu.Lock()
	defer f.mu.Unlock()

	f.conversations = append(f.conversations, record)
}

func selfSignedConfig(t *testing.T) *tls.Config {
	t.Helper()

	key, err := ecdsa.GenerateKey(elliptic.P256(), rand.Reader)
	if err != nil {
		t.Fatalf("generating a key: %v", err)
	}

	template := x509.Certificate{
		SerialNumber:          big.NewInt(1),
		Subject:               pkix.Name{CommonName: "127.0.0.1"},
		NotBefore:             time.Now().Add(-time.Hour),
		NotAfter:              time.Now().Add(time.Hour),
		KeyUsage:              x509.KeyUsageKeyEncipherment | x509.KeyUsageDigitalSignature,
		ExtKeyUsage:           []x509.ExtKeyUsage{x509.ExtKeyUsageServerAuth},
		BasicConstraintsValid: true,
		IPAddresses:           []net.IP{net.ParseIP("127.0.0.1"), net.IPv6loopback},
		DNSNames:              []string{"localhost"},
	}

	der, err := x509.CreateCertificate(rand.Reader, &template, &template, &key.PublicKey, key)
	if err != nil {
		t.Fatalf("creating a certificate: %v", err)
	}

	keyDER, err := x509.MarshalECPrivateKey(key)
	if err != nil {
		t.Fatalf("marshalling a key: %v", err)
	}

	certificate, err := tls.X509KeyPair(
		pem.EncodeToMemory(&pem.Block{Type: "CERTIFICATE", Bytes: der}),
		pem.EncodeToMemory(&pem.Block{Type: "EC PRIVATE KEY", Bytes: keyDER}),
	)
	if err != nil {
		t.Fatalf("building a key pair: %v", err)
	}

	return &tls.Config{Certificates: []tls.Certificate{certificate}, MinVersion: tls.VersionTLS12}
}
