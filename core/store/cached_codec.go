package store

import (
	"bytes"
	"crypto/sha256"
	"encoding/gob"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/farhapartex/coyote/core/model"
)

type nullValue struct{}

type cachedPage struct {
	Records []map[string]any
	Total   int64
}

func init() {
	gob.Register(nullValue{})
	gob.Register(time.Time{})
	gob.Register([]byte(nil))
	gob.Register(int(0))
	gob.Register(int32(0))
	gob.Register(int64(0))
	gob.Register(uint64(0))
	gob.Register(float32(0))
	gob.Register(float64(0))
	gob.Register(false)
	gob.Register("")
}

func encodePage(page model.Page) ([]byte, error) {
	payload := cachedPage{
		Records: make([]map[string]any, 0, len(page.Records)),
		Total:   page.Total,
	}
	for _, record := range page.Records {
		payload.Records = append(payload.Records, sealRecord(record))
	}

	buffer := &bytes.Buffer{}
	if err := gob.NewEncoder(buffer).Encode(payload); err != nil {
		return nil, fmt.Errorf("coyote/repo: caching records: %w", err)
	}
	return buffer.Bytes(), nil
}

func decodePage(raw []byte) (model.Page, error) {
	var payload cachedPage
	if err := gob.NewDecoder(bytes.NewReader(raw)).Decode(&payload); err != nil {
		return model.Page{}, fmt.Errorf("coyote/repo: reading cached records: %w", err)
	}

	page := model.Page{Total: payload.Total, Records: make([]model.Record, 0, len(payload.Records))}
	for _, record := range payload.Records {
		page.Records = append(page.Records, openRecord(record))
	}
	return page, nil
}

func sealRecord(record model.Record) map[string]any {
	out := make(map[string]any, len(record))
	for column, value := range record {
		if value == nil {
			out[column] = nullValue{}
			continue
		}
		out[column] = value
	}
	return out
}

func openRecord(sealed map[string]any) model.Record {
	out := make(model.Record, len(sealed))
	for column, value := range sealed {
		if _, isNull := value.(nullValue); isNull {
			out[column] = nil
			continue
		}
		out[column] = value
	}
	return out
}

func queryDigest(query model.Query) string {
	digest := sha256.Sum256([]byte(fmt.Sprintf("%#v", query)))
	return hex.EncodeToString(digest[:])[:16]
}
