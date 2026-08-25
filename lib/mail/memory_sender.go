package mail

import (
	"context"
	"slices"
	"sync"
)

type MemorySender struct {
	encoder  Encoder
	mu       sync.RWMutex
	messages []Message
	raw      [][]byte
}

func NewMemory() *MemorySender {
	return &MemorySender{encoder: NewEncoder()}
}

func (m *MemorySender) Send(ctx context.Context, message Message) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	raw, err := m.encoder.Encode(message)
	if err != nil {
		return err
	}

	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = append(m.messages, message.clone())
	m.raw = append(m.raw, raw)
	return nil
}

func (m *MemorySender) Sent() []Message {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([]Message, 0, len(m.messages))
	for _, entry := range m.messages {
		out = append(out, entry.clone())
	}
	return out
}

func (m *MemorySender) Raw() [][]byte {
	m.mu.RLock()
	defer m.mu.RUnlock()

	out := make([][]byte, 0, len(m.raw))
	for _, entry := range m.raw {
		out = append(out, slices.Clone(entry))
	}
	return out
}

func (m *MemorySender) Last() (Message, bool) {
	m.mu.RLock()
	defer m.mu.RUnlock()

	if len(m.messages) == 0 {
		return Message{}, false
	}
	return m.messages[len(m.messages)-1].clone(), true
}

func (m *MemorySender) Count() int {
	m.mu.RLock()
	defer m.mu.RUnlock()

	return len(m.messages)
}

func (m *MemorySender) Clear() {
	m.mu.Lock()
	defer m.mu.Unlock()

	m.messages = nil
	m.raw = nil
}
