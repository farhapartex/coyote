# Self-service accounts

[← Back to contents](README.md)

Sign-in, registration, profile editing and password change for your *users* — as opposed to the
[admin portal](15-admin.md), which is for your staff. Like the admin, it lives in `contrib/`, so it
is only compiled in if you mount it.

## Mounting

```go
accounts.Mount(a, accounts.Options{
	AllowRegistration: true,
	AfterLogin:        "/dashboard",
})
```

| Option | Default | Meaning |
| --- | --- | --- |
| `Prefix` | `/accounts` | Where the pages live |
| `AllowRegistration` | `false` | Public signup |
| `AfterLogin` | `/` | Where a successful sign-in lands |
| `AfterLogout` | `/` | Where signing out lands |
| `Pages` | built-in | Your own templates; see below |

Routes:

```
GET  /accounts/login
POST /accounts/login
POST /accounts/logout
GET  /accounts/register     only when AllowRegistration is true
POST /accounts/register
GET  /accounts/profile      signed in only
POST /accounts/profile
POST /accounts/password
```

CSRF covers all of them through the global guard, and the profile pages sit behind `RequireLogin`.

## Registration is off by default

A framework that accepts public signups without being asked is a liability, so `/accounts/register`
returns 404 until you set `AllowRegistration`. The login page only advertises signup when it is on.

New accounts are created with `IsStaff` and `IsSuperadmin` **false** — self-service can never mint
someone an admin. They are signed in immediately after registering.

Registration goes through the same service the admin uses, so
[password rules](14-authentication.md) and the uniqueness checks apply, and the reasons come back in
a form the visitor can act on:

```
That username is already taken.
password is too short; password is one of the most commonly used
```

## Sign-in

Uses `AuthenticateRequest`, so [login throttling](14-authentication.md) applies if you enabled it —
a lockout renders as 429. A `?next=` parameter is honoured through `view.SafeNext`, so it can only
send visitors to same-origin paths.

## Using your own templates

The built-in pages are deliberately plain: they work, and they are meant to be replaced. Point at
your own and they are rendered through your app's template engine, with your layout and your
styling:

```go
accounts.Mount(a, accounts.Options{
	Pages: accounts.Pages{
		Login:    "pages/login.html",
		Register: "pages/register.html",
		Profile:  "pages/profile.html",
	},
})
```

Your page gets everything a normal render does — `.User`, `.CSRFToken`, `.Flashes` — plus:

| Key | On |
| --- | --- |
| `.Error` | login, register, profile |
| `.PasswordError` | profile |
| `.Form` | register, profile — the submitted or current user |
| `.Next` | login |
| `.Prefix` | all, for building form actions |
| `.AllowRegistration` | all |

A minimal login page:

```html
{{define "content"}}
<form method="post" action="{{.Prefix}}/login">
  <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
  <input type="hidden" name="next" value="{{.Next}}">
  {{with .Error}}<p class="error">{{.}}</p>{{end}}
  <input name="username" required>
  <input type="password" name="password" required>
  <button>Sign in</button>
</form>
{{end}}
```

Set the ones you want; anything left blank keeps the built-in page.

## Changing a password

The profile page asks for the current password before accepting a new one, and the visitor stays
signed in afterwards. There is no reset-by-email flow yet — that arrives with the email layer.

## Next

- [Authentication →](14-authentication.md) — the service underneath these views
- [Permissions →](28-permissions.md)
