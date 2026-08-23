package redis

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"strconv"
	"strings"
)

const (
	typeSimple  = '+'
	typeError   = '-'
	typeInteger = ':'
	typeBulk    = '$'
	typeArray   = '*'
)

var (
	ErrProtocol = errors.New("coyote/cache/redis: unexpected reply from the server")
	ErrServer   = errors.New("coyote/cache/redis: the server refused the command")
)

type reply struct {
	kind   byte
	text   string
	bulk   []byte
	number int64
	items  []reply
	null   bool
}

func (r reply) err() error {
	if r.kind == typeError {
		return fmt.Errorf("%w: %s", ErrServer, r.text)
	}
	return nil
}

func (r reply) integer() (int64, error) {
	if err := r.err(); err != nil {
		return 0, err
	}
	if r.kind != typeInteger {
		return 0, fmt.Errorf("%w: wanted an integer, got %q", ErrProtocol, r.kind)
	}
	return r.number, nil
}

func writeCommand(w *bufio.Writer, name string, args ...[]byte) error {
	if _, err := fmt.Fprintf(w, "%c%d\r\n", typeArray, len(args)+1); err != nil {
		return err
	}
	if err := writeBulk(w, []byte(name)); err != nil {
		return err
	}
	for _, arg := range args {
		if err := writeBulk(w, arg); err != nil {
			return err
		}
	}
	return w.Flush()
}

func writeBulk(w *bufio.Writer, value []byte) error {
	if _, err := fmt.Fprintf(w, "%c%d\r\n", typeBulk, len(value)); err != nil {
		return err
	}
	if _, err := w.Write(value); err != nil {
		return err
	}
	_, err := w.WriteString("\r\n")
	return err
}

func readReply(r *bufio.Reader) (reply, error) {
	prefix, err := r.ReadByte()
	if err != nil {
		return reply{}, err
	}
	line, err := readLine(r)
	if err != nil {
		return reply{}, err
	}

	switch prefix {
	case typeSimple, typeError:
		return reply{kind: prefix, text: line}, nil
	case typeInteger:
		number, err := strconv.ParseInt(line, 10, 64)
		if err != nil {
			return reply{}, fmt.Errorf("%w: %q is not an integer", ErrProtocol, line)
		}
		return reply{kind: prefix, number: number}, nil
	case typeBulk:
		return readBulk(r, line)
	case typeArray:
		return readArray(r, line)
	}
	return reply{}, fmt.Errorf("%w: unknown reply type %q", ErrProtocol, prefix)
}

func readBulk(r *bufio.Reader, line string) (reply, error) {
	size, err := strconv.Atoi(line)
	if err != nil {
		return reply{}, fmt.Errorf("%w: %q is not a length", ErrProtocol, line)
	}
	if size < 0 {
		return reply{kind: typeBulk, null: true}, nil
	}
	buffer := make([]byte, size+2)
	if _, err := io.ReadFull(r, buffer); err != nil {
		return reply{}, err
	}
	return reply{kind: typeBulk, bulk: buffer[:size]}, nil
}

func readArray(r *bufio.Reader, line string) (reply, error) {
	count, err := strconv.Atoi(line)
	if err != nil {
		return reply{}, fmt.Errorf("%w: %q is not a count", ErrProtocol, line)
	}
	if count < 0 {
		return reply{kind: typeArray, null: true}, nil
	}
	items := make([]reply, 0, count)
	for range count {
		item, err := readReply(r)
		if err != nil {
			return reply{}, err
		}
		items = append(items, item)
	}
	return reply{kind: typeArray, items: items}, nil
}

func readLine(r *bufio.Reader) (string, error) {
	line, err := r.ReadString('\n')
	if err != nil {
		return "", err
	}
	return strings.TrimRight(line, "\r\n"), nil
}
