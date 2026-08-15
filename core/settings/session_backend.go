package settings

type SessionBackend string

const (
	SessionsInMemory SessionBackend = "memory"
	SessionsInDB     SessionBackend = "database"
)

func (b SessionBackend) Persistent() bool { return b == SessionsInDB }
