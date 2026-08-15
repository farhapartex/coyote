# First run

[← Back to contents](README.md)

A fresh install has no schema and no users, which means nobody can sign in yet. The framework says
so on startup — a real check against the migration ledger, not a first-boot flag.

```
$ go tool coyote start
WARN  this database has no schema yet and no migrations are declared; run: go tool coyote makemigrations && go tool coyote migrate
INFO  coyote listening addr=127.0.0.1:8000
```

Once migrations exist but have not been applied it says that instead, and once applied it checks
whether anyone can actually sign in:

```
WARN  no migrations have been applied to this database; nothing will work until you run: go tool coyote migrate pending=1
WARN  migrations are pending; run: go tool coyote migrate pending=1 applied=2
WARN  there are no users yet, so nobody can sign in; run: go tool coyote createsuperadmin
INFO  migrations up to date applied=3
```

The server still starts in every case — an unreachable database downgrades to a warning rather than
refusing to boot, so a misconfigured database does not take the whole site down before you can read
the message.

## The sequence

```
go tool coyote makemigrations --name=initial
go tool coyote migrate
go tool coyote createsuperadmin
go tool coyote start
```

`migrate` refuses unless the server is already listening on the configured address, so start it
first in another terminal:

```
$ go tool coyote migrate
coyote/cli: server is not running at 127.0.0.1:8000; start it first with: go tool coyote start
```

## createsuperadmin

```
go tool coyote createsuperadmin
go tool coyote createsuperadmin --username=root --email=root@site.com --password=secret
COYOTE_SUPERADMIN_PASSWORD=secret go tool coyote createsuperadmin --username=root
```

Flags win, then the `COYOTE_SUPERADMIN_*` environment variables, then an interactive prompt for
whatever is still missing. The password is **echoed** while you type — use the environment variable
form in scripts and on shared terminals.

The command refuses to run before the `users` table exists:

```
coyote/cli: the users table does not exist yet; run: go tool coyote makemigrations && go tool coyote migrate
```

Users are created through the same service the admin portal uses, so passwords are hashed by one
code path.

## Next

[Deployment →](25-deployment.md)
