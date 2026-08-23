package tests

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"path"
	"sort"
	"strconv"
	"strings"
	"sync"
	"testing"
	"time"
)

type fakeEntry struct {
	value   []byte
	expires time.Time
}

type fakeRedis struct {
	listener net.Listener
	mu       sync.Mutex
	values   map[string]fakeEntry
	seen     []string
	password string
	failWith string
	dropOn   string
	delay    time.Duration
	scanStep int
	closed   bool
}

func newFakeRedis(t *testing.T) *fakeRedis {
	t.Helper()
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listening: %v", err)
	}
	server := &fakeRedis{
		listener: listener,
		values:   map[string]fakeEntry{},
		scanStep: 2,
	}
	go server.accept()
	t.Cleanup(server.Close)
	return server
}

func (f *fakeRedis) Address() string { return f.listener.Addr().String() }

func (f *fakeRedis) Close() {
	f.mu.Lock()
	if f.closed {
		f.mu.Unlock()
		return
	}
	f.closed = true
	f.mu.Unlock()
	_ = f.listener.Close()
}

func (f *fakeRedis) Commands() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	return append([]string{}, f.seen...)
}

func (f *fakeRedis) FailWith(message string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.failWith = message
}

func (f *fakeRedis) DropOn(command string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.dropOn = strings.ToUpper(command)
}

func (f *fakeRedis) DelayBy(delay time.Duration) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.delay = delay
}

func (f *fakeRedis) RequirePassword(password string) {
	f.mu.Lock()
	defer f.mu.Unlock()
	f.password = password
}

func (f *fakeRedis) Keys() []string {
	f.mu.Lock()
	defer f.mu.Unlock()
	out := make([]string, 0, len(f.values))
	for key := range f.values {
		out = append(out, key)
	}
	sort.Strings(out)
	return out
}

func (f *fakeRedis) accept() {
	for {
		conn, err := f.listener.Accept()
		if err != nil {
			return
		}
		go f.serve(conn)
	}
}

func (f *fakeRedis) serve(conn net.Conn) {
	defer conn.Close()
	reader := bufio.NewReader(conn)
	writer := bufio.NewWriter(conn)

	for {
		args, err := readFakeCommand(reader)
		if err != nil {
			return
		}
		if len(args) == 0 {
			return
		}
		name := strings.ToUpper(args[0])

		f.mu.Lock()
		f.seen = append(f.seen, name)
		delay, failWith, dropOn := f.delay, f.failWith, f.dropOn
		f.mu.Unlock()

		if delay > 0 {
			time.Sleep(delay)
		}
		if dropOn != "" && dropOn == name {
			f.mu.Lock()
			f.dropOn = ""
			f.mu.Unlock()
			return
		}
		if failWith != "" && name != "AUTH" && name != "SELECT" {
			writeFakeError(writer, failWith)
			continue
		}
		if err := f.dispatch(writer, name, args[1:]); err != nil {
			return
		}
	}
}

func (f *fakeRedis) dispatch(w *bufio.Writer, name string, args []string) error {
	switch name {
	case "PING":
		return writeFakeSimple(w, "PONG")
	case "AUTH":
		return f.auth(w, args)
	case "SELECT":
		return writeFakeSimple(w, "OK")
	case "SET":
		return f.set(w, args)
	case "GET":
		return f.get(w, args)
	case "DEL":
		return f.del(w, args)
	case "EXISTS":
		return f.exists(w, args)
	case "MGET":
		return f.mget(w, args)
	case "INCRBY":
		return f.incrby(w, args)
	case "PEXPIRE":
		return f.pexpire(w, args)
	case "SCAN":
		return f.scan(w, args)
	}
	return writeFakeError(w, "ERR unknown command '"+name+"'")
}

func (f *fakeRedis) auth(w *bufio.Writer, args []string) error {
	f.mu.Lock()
	want := f.password
	f.mu.Unlock()

	if want == "" || (len(args) > 0 && args[len(args)-1] == want) {
		return writeFakeSimple(w, "OK")
	}
	return writeFakeError(w, "WRONGPASS invalid password")
}

func (f *fakeRedis) set(w *bufio.Writer, args []string) error {
	if len(args) < 2 {
		return writeFakeError(w, "ERR wrong number of arguments")
	}
	entry := fakeEntry{value: []byte(args[1])}
	for i := 2; i+1 < len(args); i += 2 {
		if strings.EqualFold(args[i], "PX") {
			milliseconds, err := strconv.ParseInt(args[i+1], 10, 64)
			if err != nil {
				return writeFakeError(w, "ERR value is not an integer or out of range")
			}
			entry.expires = time.Now().Add(time.Duration(milliseconds) * time.Millisecond)
		}
	}

	f.mu.Lock()
	f.values[args[0]] = entry
	f.mu.Unlock()
	return writeFakeSimple(w, "OK")
}

func (f *fakeRedis) get(w *bufio.Writer, args []string) error {
	if len(args) != 1 {
		return writeFakeError(w, "ERR wrong number of arguments")
	}
	value, found := f.lookup(args[0])
	if !found {
		return writeFakeNull(w)
	}
	return writeFakeBulk(w, value)
}

func (f *fakeRedis) del(w *bufio.Writer, args []string) error {
	removed := 0
	f.mu.Lock()
	for _, key := range args {
		if _, found := f.values[key]; found {
			delete(f.values, key)
			removed++
		}
	}
	f.mu.Unlock()
	return writeFakeInteger(w, int64(removed))
}

func (f *fakeRedis) exists(w *bufio.Writer, args []string) error {
	total := 0
	for _, key := range args {
		if _, found := f.lookup(key); found {
			total++
		}
	}
	return writeFakeInteger(w, int64(total))
}

func (f *fakeRedis) mget(w *bufio.Writer, args []string) error {
	if err := writeFakeArrayHeader(w, len(args)); err != nil {
		return err
	}
	for _, key := range args {
		value, found := f.lookup(key)
		if !found {
			if err := writeFakeNull(w); err != nil {
				return err
			}
			continue
		}
		if err := writeFakeBulk(w, value); err != nil {
			return err
		}
	}
	return w.Flush()
}

func (f *fakeRedis) incrby(w *bufio.Writer, args []string) error {
	if len(args) != 2 {
		return writeFakeError(w, "ERR wrong number of arguments")
	}
	delta, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return writeFakeError(w, "ERR value is not an integer or out of range")
	}

	current := int64(0)
	if raw, found := f.lookup(args[0]); found {
		if parsed, err := strconv.ParseInt(string(raw), 10, 64); err == nil {
			current = parsed
		} else {
			return writeFakeError(w, "ERR value is not an integer or out of range")
		}
	}
	current += delta

	f.mu.Lock()
	existing := f.values[args[0]]
	f.values[args[0]] = fakeEntry{value: strconv.AppendInt(nil, current, 10), expires: existing.expires}
	f.mu.Unlock()
	return writeFakeInteger(w, current)
}

func (f *fakeRedis) pexpire(w *bufio.Writer, args []string) error {
	if len(args) != 2 {
		return writeFakeError(w, "ERR wrong number of arguments")
	}
	milliseconds, err := strconv.ParseInt(args[1], 10, 64)
	if err != nil {
		return writeFakeError(w, "ERR value is not an integer or out of range")
	}

	f.mu.Lock()
	defer f.mu.Unlock()
	entry, found := f.values[args[0]]
	if !found {
		return writeFakeInteger(w, 0)
	}
	entry.expires = time.Now().Add(time.Duration(milliseconds) * time.Millisecond)
	f.values[args[0]] = entry
	return writeFakeInteger(w, 1)
}

func (f *fakeRedis) scan(w *bufio.Writer, args []string) error {
	if len(args) == 0 {
		return writeFakeError(w, "ERR wrong number of arguments")
	}
	cursor := args[0]

	pattern := "*"
	for i := 1; i+1 < len(args); i += 2 {
		if strings.EqualFold(args[i], "MATCH") {
			pattern = args[i+1]
		}
	}

	f.mu.Lock()
	step := f.scanStep
	f.mu.Unlock()

	page := []string{}
	for _, key := range f.Keys() {
		if cursor != "0" && key <= cursor {
			continue
		}
		if ok, err := path.Match(pattern, key); err != nil || !ok {
			continue
		}
		page = append(page, key)
		if len(page) == step {
			break
		}
	}

	next := "0"
	if len(page) == step {
		next = page[len(page)-1]
	}

	if err := writeFakeArrayHeader(w, 2); err != nil {
		return err
	}
	if err := writeFakeBulk(w, []byte(next)); err != nil {
		return err
	}
	if err := writeFakeArrayHeader(w, len(page)); err != nil {
		return err
	}
	for _, key := range page {
		if err := writeFakeBulk(w, []byte(key)); err != nil {
			return err
		}
	}
	return w.Flush()
}

func (f *fakeRedis) lookup(key string) ([]byte, bool) {
	f.mu.Lock()
	defer f.mu.Unlock()
	entry, found := f.values[key]
	if !found {
		return nil, false
	}
	if !entry.expires.IsZero() && time.Now().After(entry.expires) {
		delete(f.values, key)
		return nil, false
	}
	return entry.value, true
}

func readFakeCommand(r *bufio.Reader) ([]string, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return nil, err
	}
	if prefix != '*' {
		return nil, io.ErrUnexpectedEOF
	}
	line, err := readFakeLine(r)
	if err != nil {
		return nil, err
	}
	count, err := strconv.Atoi(line)
	if err != nil {
		return nil, err
	}

	args := make([]string, 0, count)
	for range count {
		if _, err := r.ReadByte(); err != nil {
			return nil, err
		}
		sizeLine, err := readFakeLine(r)
		if err != nil {
			return nil, err
		}
		size, err := strconv.Atoi(sizeLine)
		if err != nil {
			return nil, err
		}
		buffer := make([]byte, size+2)
		if _, err := io.ReadFull(r, buffer); err != nil {
			return nil, err
		}
		args = append(args, string(buffer[:size]))
	}
	return args, nil
}

func readFakeLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}

func writeFakeSimple(w *bufio.Writer, value string) error {
	if _, err := fmt.Fprintf(w, "+%s\r\n", value); err != nil {
		return err
	}
	return w.Flush()
}

func writeFakeError(w *bufio.Writer, message string) error {
	if _, err := fmt.Fprintf(w, "-%s\r\n", message); err != nil {
		return err
	}
	return w.Flush()
}

func writeFakeInteger(w *bufio.Writer, value int64) error {
	if _, err := fmt.Fprintf(w, ":%d\r\n", value); err != nil {
		return err
	}
	return w.Flush()
}

func writeFakeBulk(w *bufio.Writer, value []byte) error {
	if _, err := fmt.Fprintf(w, "$%d\r\n", len(value)); err != nil {
		return err
	}
	if _, err := w.Write(value); err != nil {
		return err
	}
	if _, err := w.WriteString("\r\n"); err != nil {
		return err
	}
	return w.Flush()
}

func writeFakeNull(w *bufio.Writer) error {
	if _, err := w.WriteString("$-1\r\n"); err != nil {
		return err
	}
	return w.Flush()
}

func writeFakeArrayHeader(w *bufio.Writer, count int) error {
	_, err := fmt.Fprintf(w, "*%d\r\n", count)
	return err
}
