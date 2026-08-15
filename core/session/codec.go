package session

import (
	"bytes"
	"encoding/gob"
	"fmt"
	"time"
)

func init() {
	RegisterValue([]Flash{})
	RegisterValue(map[string]any{})
	RegisterValue([]any{})
	RegisterValue([]string{})
	RegisterValue([]int{})
	RegisterValue(time.Time{})
}

func RegisterValue(value any) {
	gob.Register(value)
}

func EncodeValues(values map[string]any) ([]byte, error) {
	buffer := &bytes.Buffer{}
	if err := gob.NewEncoder(buffer).Encode(values); err != nil {
		return nil, fmt.Errorf("coyote/session: encoding values: %w", err)
	}
	return buffer.Bytes(), nil
}

func DecodeValues(raw []byte) (map[string]any, error) {
	values := map[string]any{}
	if len(raw) == 0 {
		return values, nil
	}
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&values); err != nil {
		return nil, fmt.Errorf("coyote/session: decoding values: %w", err)
	}
	return values, nil
}
