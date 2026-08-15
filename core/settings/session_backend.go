package settings

type SessionBackend string

const (
	SessionsInMemory SessionBackend = "memory"
	SessionsInDB     SessionBackend = "database"
	SessionsInCookie SessionBackend = "cookie"
)

func (b SessionBackend) Persistent() bool { return b == SessionsInDB }

func (b SessionBackend) Stateless() bool { return b == SessionsInCookie }
