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
	EnvFake     = "COYOTE_MIGRATE_FAKE"
	EnvTarget   = "COYOTE_MIGRATE_TARGET"
	EnvSteps    = "COYOTE_MIGRATE_STEPS"
	EnvNoInput  = "COYOTE_NO_INPUT"
	EnvForce    = "COYOTE_FORCE"
)

const (
	NameStart           = "start"
	NameMigrate         = "migrate"
	NameSyncPermissions = "syncpermissions"
	NameCollectStatic   = "collectstatic"
)

func Requested() string {
	if command := os.Getenv(EnvCommand); command != "" {
		return command
	}
	return NameStart
}
