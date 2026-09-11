package main

import (
	"strconv"

	"github.com/farhapartex/coyote/core/model"
)

func recordInt(record model.Record, column string) int64 {
	switch typed := record.Get(column).(type) {
	case int64:
		return typed
	case int:
		return int64(typed)
	case int32:
		return int64(typed)
	case float64:
		return int64(typed)
	case string:
		number, _ := strconv.ParseInt(typed, 10, 64)
		return number
	case []byte:
		number, _ := strconv.ParseInt(string(typed), 10, 64)
		return number
	}
	return 0
}
