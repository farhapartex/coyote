package model

import "strings"

var sensitiveNames = map[string]bool{
	"password": true,
	"secret":   true,
	"token":    true,
	"api_key":  true,
	"salt":     true,
	"pin":      true,
}

var sensitiveEndings = []string{"_password", "_secret", "_token", "_hash", "_key"}

var sensitiveBeginnings = []string{"password", "otp"}

func isSensitive(column string) bool {
	if sensitiveNames[column] {
		return true
	}
	for _, ending := range sensitiveEndings {
		if strings.HasSuffix(column, ending) {
			return true
		}
	}
	for _, beginning := range sensitiveBeginnings {
		if strings.HasPrefix(column, beginning) {
			return true
		}
	}
	return false
}
