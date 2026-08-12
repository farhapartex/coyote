package migrate

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type Migration struct {
	ID   string
	Note string
	Up   []Op
}

func (m Migration) Checksum() string {
	digest := sha256.New()
	for _, op := range m.Up {
		digest.Write([]byte(op.Describe()))
		digest.Write([]byte{'\n'})
	}
	return hex.EncodeToString(digest.Sum(nil))[:16]
}

func (m Migration) Describe() []string {
	out := make([]string, 0, len(m.Up))
	for _, op := range m.Up {
		out = append(out, op.Describe())
	}
	return out
}

func (m Migration) Title() string {
	name := strings.TrimSpace(m.Note)
	if name == "" {
		return m.ID
	}
	return m.ID + " (" + name + ")"
}
