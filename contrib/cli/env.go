package cli

import "os"

const (
	EnvCommand = "COYOTE_COMMAND"
	EnvPort    = "COYOTE_PORT"
	EnvHost    = "COYOTE_HOST"
)

const (
	NameStart   = "start"
	NameMigrate = "migrate"
)

func Requested() string {
	if command := os.Getenv(EnvCommand); command != "" {
		return command
	}
	return NameStart
}
