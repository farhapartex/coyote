package model

import "strings"

var sensitiveNames = map[string]bool{
	"password": true,
	"secret":   true,
	"token":    true,
	"api_key":  true,
}

func isSensitive(column string) bool {
	if sensitiveNames[column] {
		return true
	}
	return strings.HasSuffix(column, "_password") || strings.HasSuffix(column, "_secret")
}
