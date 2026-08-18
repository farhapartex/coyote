package cli

import (
	"os"
	"strconv"
	"strings"
)

func flagSet(name string) string {
	return strings.TrimSpace(os.Getenv(name))
}

func flagBool(name string) bool {
	switch strings.ToLower(flagSet(name)) {
	case "1", "true", "yes", "on":
		return true
	}
	return false
}

func flagInt(name string, fallback int) int {
	if value, err := strconv.Atoi(flagSet(name)); err == nil && value > 0 {
		return value
	}
	return fallback
}
