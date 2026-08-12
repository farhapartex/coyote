package dialect

import "github.com/farhapartex/coyote/core/model"

func TypeFor(d Dialect, kind model.Kind, size int, autoIncrement bool) string {
	switch d.Name() {
	case "postgres":
		return PostgresType(kind, size, autoIncrement)
	case "mysql":
		return MySQLType(kind, size, autoIncrement)
	default:
		return SQLiteType(kind, autoIncrement)
	}
}
