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
	IsStaff      bool   `gorm:"index"`
	IsSuperadmin bool   `gorm:"index"`
	LastLoginAt  time.Time
	CreatedAt    time.Time
	UpdatedAt    time.Time
}
```

Helpers: `FullName()`, `DisplayName()` (falls back to the username), `Initials()`, `HasLoggedIn()`,
`HasUsablePassword()`, `Clone()`, `Validate()`.

Two role flags, and they mean different things:

| Flag | Grants |
| --- | --- |
| `IsStaff` | can sign in to the admin portal |
| `IsSuperadmin` | everything, including user and session management |

A superadmin always counts as staff — setting one sets the other, so a superadmin can never be
locked out of the portal by accident. A plain user has neither and is refused at the login screen.

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
coyote createsuperadmin
```

## Passwords

PBKDF2-SHA256 with `Auth.PBKDF2Iterations` (600,000 by default) and a 16-byte random salt, stored in
the Django-style `pbkdf2_sha256$iterations$salt$hash` format.

### Strength rules

A password is checked against a list of rules before it is ever hashed. Four apply by default:

| Rule | Rejects |
| --- | --- |
| `MinimumLength(n)` | shorter than `Auth.PasswordMinLength` |
| `NotEntirelyNumeric()` | `19850413` |
| `NotSimilarToAccount()` | anything containing the username, email local part, or name |
| `NotCommon(nil)` | entries in the built-in common-password list |

Every failing rule is reported at once, so the user fixes one password instead of failing four
times:

```
coyote/auth: password rejected: coyote/auth: password is too short
coyote/auth: password is entirely numeric
coyote/auth: password is one of the most commonly used
```

Each is an ordinary error, so `errors.Is(err, auth.ErrPasswordCommon)` works for rendering a
specific message.

Add your own by appending to the defaults:

```go
s.Auth.PasswordRules = append(auth.DefaultPasswordRules(s.Auth.PasswordMinLength),
	func(password string, u *auth.User) error {
		if !strings.ContainsAny(password, "0123456789") {
			return errors.New("needs at least one digit")
		}
		return nil
	},
)
```

Assigning a list **replaces** the defaults rather than adding to them, which is how you relax the
policy:

```go
s.Auth.PasswordRules = []auth.PasswordRule{auth.MinimumLength(4)}   // tests only
```

The built-in common-password list is deliberately small. Swap in a bigger one:

```go
auth.NotCommon(myTwentyThousandEntries)
```

Rules run wherever a password is set — `CreateUser`, `CreateSuperadmin`, `SetPassword`, the admin
portal's password field, and `createsuperadmin`. They never run at login, so tightening the policy
does not lock out existing accounts.

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

## Login throttling

Off by default. Turn it on and repeated failures lock the pair out:

```go
s.Auth.Throttle = auth.ThrottlePolicy{
	Enabled:     true,
	MaxAttempts: 5,
	Window:      15 * time.Minute,
	Lockout:     15 * time.Minute,
}
```

- The key is **username plus client IP**, so someone guessing at your name from their own machine
  cannot lock you out of yours.
- An unknown username is throttled exactly like a real one. If it were not, the lockout itself would
  reveal which accounts exist.
- A successful sign-in clears the counter.
- `Authenticate` alone keys on the username; `AuthenticateRequest(r, …)` adds the IP. The admin
  portal uses the second.
- Over the limit returns `auth.ErrTooManyAttempts`; the admin login renders it as a 429.

`IsActive = false` is a separate, permanent block — throttling only limits the rate of attempts.

Counters live in memory and are swept, so the map does not grow forever. That also means **each
process has its own counters**: behind several instances the effective allowance multiplies. Supply
your own `auth.LoginLimiter` (`Allow`, `Fail`, `Reset`) to share them.

## Guarding routes

```go
login := a.Settings.Auth.LoginURL

me    := a.Group("/me",    a.Auth.RequireLogin(login))
tools := a.Group("/tools", a.Auth.RequireStaff(login))
root  := a.Group("/root",  a.Auth.RequireSuperadmin(login))
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
