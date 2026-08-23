package cache

import (
	"encoding/json"
	"fmt"
)

type JSONCodec struct{}

func (JSONCodec) Encode(value any) ([]byte, error) {
	raw, err := json.Marshal(value)
	if err != nil {
		return nil, fmt.Errorf("coyote/cache: encoding %T: %w", value, err)
	}
	return raw, nil
}

func (JSONCodec) Decode(raw []byte, target any) error {
	if err := json.Unmarshal(raw, target); err != nil {
		return fmt.Errorf("coyote/cache: decoding %T: %w", target, err)
	}
	return nil
}
