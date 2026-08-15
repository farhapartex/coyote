package session

import (
	"bytes"
	"encoding/gob"
	"time"
)

type carrier interface {
	load(value string) (*Session, bool)
	persist(s *Session) (string, error)
	forget(id string) error
}

type storeCarrier struct {
	store Store
}

func (c storeCarrier) load(value string) (*Session, bool) { return c.store.Load(value) }

func (c storeCarrier) persist(s *Session) (string, error) {
	if err := c.store.Save(s); err != nil {
		return "", err
	}
	return s.ID(), nil
}

func (c storeCarrier) forget(id string) error { return c.store.Delete(id) }

type sealedSession struct {
	ID      string
	Values  map[string]any
	Created time.Time
	Expires time.Time
}

type cookieCarrier struct {
	sealer *Sealer
}

func (c cookieCarrier) load(value string) (*Session, bool) {
	payload, err := c.sealer.Open(value)
	if err != nil {
		return nil, false
	}
	var decoded sealedSession
	if err := gob.NewDecoder(bytes.NewReader(payload)).Decode(&decoded); err != nil {
		return nil, false
	}
	if time.Now().After(decoded.Expires) {
		return nil, false
	}
	return Restore(decoded.ID, decoded.Values, decoded.Created, decoded.Expires), true
}

func (c cookieCarrier) persist(s *Session) (string, error) {
	buffer := &bytes.Buffer{}
	payload := sealedSession{
		ID:      s.ID(),
		Values:  s.Values(),
		Created: s.CreatedAt(),
		Expires: s.ExpiresAt(),
	}
	if err := gob.NewEncoder(buffer).Encode(payload); err != nil {
		return "", err
	}
	return c.sealer.Seal(buffer.Bytes())
}

func (c cookieCarrier) forget(string) error { return nil }
