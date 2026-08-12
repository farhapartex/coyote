package cli

import "os"

const (
	EnvCommand  = "COYOTE_COMMAND"
	EnvPort     = "COYOTE_PORT"
	EnvHost     = "COYOTE_HOST"
	EnvName     = "COYOTE_MIGRATION_NAME"
	EnvUsername = "COYOTE_SUPERADMIN_USERNAME"
	EnvEmail    = "COYOTE_SUPERADMIN_EMAIL"
	EnvPassword = "COYOTE_SUPERADMIN_PASSWORD"
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
