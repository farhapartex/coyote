# Authentication

[← Back to contents](README.md)

A user entity, password hashing, sign-in, and route guards come with the framework. Authentication
state lives in the [session](13-sessions.md); the user lives in the database.

## The user entity

`auth.User` is what you get on install:

```go
type User struct {
	ID           string `gorm:"primaryKey;size:64"`
	FirstName    string
	LastName     string
	Email        string `gorm:"index;size:320"`
	Username     string `gorm:"uniqueIndex;size:64;not null"`
	Password     string `gorm:"not null"`
	IsActive     bool   `gorm:"index;default:true"`
	IsSuperadmin bool   `gorm:"index"`
	LastLoginAt  time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
```

Helpers: `FullName()`, `DisplayName()` (falls back to the username), `Initials()`, `HasLoggedIn()`,
`HasUsablePassword()`, `Clone()`, `Validate()`.

`IsSuperadmin` is the only role flag: it grants full permissions and access to the admin portal.
There is no separate staff tier — add your own field if you want an intermediate role.

Store rules: usernames and emails are unique case-insensitively, a blank email is allowed and does
not collide, and the last active superadmin cannot be deleted, demoted, or disabled.

## Creating users

Always through the service, so hashing and defaults are never skipped:

```go
user, err := a.Auth.CreateUser(auth.NewUser{
	Username:  "jane",
	Email:     "jane@example.com",
	FirstName: "Jane",
	LastName:  "Doe",
	Password:  "supersecret",
})

root, err := a.Auth.CreateSuperadmin("root", "root@example.com", "supersecret")
```

New accounts are active, and not superadmin unless asked. For the first account on a fresh install
use the CLI instead — see [First run](24-first-run.md):

```
go tool coyote createsuperadmin
```

## Passwords

PBKDF2-SHA256 with `Auth.PBKDF2Iterations` (600,000 by default) and a 16-byte random salt, stored in
the Django-style `pbkdf2_sha256$iterations$salt$hash` format.

`Password` always holds a hash. The service is the only thing that writes it, and `VerifyPassword`
rejects anything that is not in hash format — so a plain string assigned by mistake fails closed
rather than authenticating.

```go
a.Auth.SetPassword(userID, "new-password")
a.Auth.ValidatePassword("candidate")      // length policy
a.Auth.MinPasswordLength()
```

Lower the iteration count in tests; leave it alone in production.

## Signing in and out

```go
user, err := a.Auth.Authenticate(username, password)
if err != nil {
	view.Error(r, "Wrong username or password.")
	view.Redirect(w, r, "/login")
	return
}

a.Auth.Login(r, user)     // rotates the session id
```

```go
a.Auth.Logout(r)
a.Auth.CurrentUser(r)     // *auth.User, or nil
```

In templates the same user is `.User`.

Login on an unknown username still runs a hash, so response timing does not reveal which usernames
exist.

## Guarding routes

```go
login := a.Settings.Auth.LoginURL

me   := a.Group("/me",   a.Auth.RequireLogin(login))
root := a.Group("/root", a.Auth.RequireSuperadmin(login))
```

Anonymous visitors are redirected to the login page with a `?next=` parameter; signed-in users who
lack the role get a 403. Send them back safely afterwards with `view.SafeNext` — see
[Views](07-views.md).

Guards work on a single route too:

```go
a.Get("/account", account, a.Auth.RequireLogin(login))
```

## Replacing or extending the user

Embed the entity when you only need extra fields beside it:

```go
type Employee struct {
	auth.User
	Department string
	ManagerID  string
}
```

Or implement `auth.Store` over your own table and set it in settings:

```go
type Store interface {
	ByID(id string) (*User, error)
	ByUsername(username string) (*User, error)
	ByEmail(email string) (*User, error)
	Create(u *User) error
	Update(u *User) error
	Delete(id string) error
	All() []*User
	Count() int
}
```

```go
s.Auth.UserStore = myLDAPStore{}
```

`auth.Guarded(store)` wraps any store with the superadmin-protection rules, so a custom backend
keeps them.

A fully swappable user model — the equivalent of Django's `AUTH_USER_MODEL` — is not here yet; the
framework's own code still works in terms of `*auth.User`.

## Next

[Admin portal →](15-admin.md)
