package jobs

import (
	"encoding/json"
	"fmt"
)

func encodePayload(args any) ([]byte, error) {
	if args == nil {
		return nil, nil
	}
	raw, err := json.Marshal(args)
	if err != nil {
		return nil, fmt.Errorf("coyote/jobs: encoding the payload: %w", err)
	}
	return raw, nil
}

func decodePayload[T any](kind string, raw []byte) (T, error) {
	var args T
	if len(raw) == 0 {
		return args, nil
	}
	if err := json.Unmarshal(raw, &args); err != nil {
		return args, fmt.Errorf("%w: %s: %w", ErrPayloadInvalid, kind, err)
	}
	return args, nil
}
