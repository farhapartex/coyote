package redis

import (
	"context"
	"strconv"
	"strings"
	"time"
)

type Store struct {
	pool  *pool
	count int
}

func New(opts Options) *Store {
	resolved := opts.withDefaults()
	return &Store{pool: newPool(resolved), count: DefaultScanCount}
}

func (s *Store) Get(ctx context.Context, key string) ([]byte, bool, error) {
	answer, err := s.command(ctx, "GET", []byte(key))
	if err != nil {
		return nil, false, err
	}
	if answer.null {
		return nil, false, nil
	}
	return answer.bulk, true, nil
}

func (s *Store) Set(ctx context.Context, key string, value []byte, ttl time.Duration) error {
	args := [][]byte{[]byte(key), value}
	if ttl > 0 {
		args = append(args, []byte("PX"), strconv.AppendInt(nil, ttl.Milliseconds(), 10))
	}
	_, err := s.command(ctx, "SET", args...)
	return err
}

func (s *Store) Delete(ctx context.Context, key string) error {
	_, err := s.command(ctx, "DEL", []byte(key))
	return err
}

func (s *Store) Has(ctx context.Context, key string) (bool, error) {
	answer, err := s.command(ctx, "EXISTS", []byte(key))
	if err != nil {
		return false, err
	}
	total, err := answer.integer()
	return total > 0, err
}

func (s *Store) Clear(ctx context.Context) error {
	return s.ClearPrefix(ctx, "")
}

func (s *Store) ClearPrefix(ctx context.Context, prefix string) error {
	pattern := []byte(escapeGlob(prefix) + "*")

	for range maxClearPasses {
		deleted, err := s.clearPass(ctx, pattern)
		if err != nil {
			return err
		}
		if deleted == 0 {
			return nil
		}
	}
	return nil
}

func (s *Store) clearPass(ctx context.Context, pattern []byte) (int, error) {
	cursor := "0"
	deleted := 0

	for {
		answer, err := s.command(ctx, "SCAN",
			[]byte(cursor), []byte("MATCH"), pattern, []byte("COUNT"),
			strconv.AppendInt(nil, int64(s.count), 10))
		if err != nil {
			return deleted, err
		}
		if len(answer.items) != 2 {
			return deleted, ErrProtocol
		}

		next := string(answer.items[0].bulk)
		keys := answer.items[1].items
		if len(keys) > 0 {
			args := make([][]byte, 0, len(keys))
			for _, key := range keys {
				args = append(args, key.bulk)
			}
			if _, err := s.command(ctx, "DEL", args...); err != nil {
				return deleted, err
			}
			deleted += len(keys)
		}
		if next == "0" || next == "" {
			return deleted, nil
		}
		cursor = next
	}
}

func (s *Store) Incr(ctx context.Context, key string, delta int64, ttl time.Duration) (int64, error) {
	answer, err := s.command(ctx, "INCRBY", []byte(key), strconv.AppendInt(nil, delta, 10))
	if err != nil {
		return 0, err
	}
	current, err := answer.integer()
	if err != nil {
		return 0, err
	}
	if ttl > 0 {
		if _, err := s.command(ctx, "PEXPIRE", []byte(key),
			strconv.AppendInt(nil, ttl.Milliseconds(), 10)); err != nil {
			return current, err
		}
	}
	return current, nil
}

func (s *Store) GetMulti(ctx context.Context, keys []string) (map[string][]byte, error) {
	if len(keys) == 0 {
		return map[string][]byte{}, nil
	}
	args := make([][]byte, 0, len(keys))
	for _, key := range keys {
		args = append(args, []byte(key))
	}

	answer, err := s.command(ctx, "MGET", args...)
	if err != nil {
		return nil, err
	}
	out := make(map[string][]byte, len(keys))
	for i, item := range answer.items {
		if i >= len(keys) || item.null {
			continue
		}
		out[keys[i]] = item.bulk
	}
	return out, nil
}

func (s *Store) Ping(ctx context.Context) error {
	_, err := s.command(ctx, "PING")
	return err
}

func (s *Store) Len() int { return -1 }

func (s *Store) Close() error { return s.pool.close() }

func (s *Store) command(ctx context.Context, name string, args ...[]byte) (reply, error) {
	var lastErr error

	for attempt := range 2 {
		c, err := s.pool.acquire(ctx)
		if err != nil {
			return reply{}, err
		}
		answer, err := c.do(ctx, name, args...)
		broken := c.broken
		s.pool.release(c)

		if err == nil {
			return answer, nil
		}
		lastErr = err
		if !broken || attempt == 1 {
			return reply{}, err
		}
	}
	return reply{}, lastErr
}

func escapeGlob(value string) string {
	replacer := strings.NewReplacer(
		`\`, `\\`,
		"*", `\*`,
		"?", `\?`,
		"[", `\[`,
		"]", `\]`,
	)
	return replacer.Replace(value)
}
