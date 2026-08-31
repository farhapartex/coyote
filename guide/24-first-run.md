# First run

[← Back to contents](README.md)

A fresh install has no schema and no users, which means nobody can sign in yet. The framework says
so on startup — a real check against the migration ledger, not a first-boot flag.

```
$ coyote start
WARN  this database has no schema yet and no migrations are declared; run: coyote makemigrations && coyote migrate
INFO  coyote listening addr=127.0.0.1:8000
```

Once migrations exist but have not been applied it says that instead, and once applied it checks
whether anyone can actually sign in:

```
WARN  no migrations have been applied to this database; nothing will work until you run: coyote migrate pending=1
WARN  migrations are pending; run: coyote migrate pending=1 applied=2
WARN  there are no users yet, so nobody can sign in; run: coyote createsuperadmin
INFO  migrations up to date applied=3
```

The server still starts in every case — an unreachable database downgrades to a warning rather than
refusing to boot, so a misconfigured database does not take the whole site down before you can read
the message.

A **cache** is the exception. If you configured one and it cannot be reached, the server refuses to
start, because configuring Redis is a statement that Redis exists:

```
$ coyote start
coyote: coyote/cache: the "default" cache is unreachable
    backend redis at 127.0.0.1:6379

  start Redis, or set Caches[0].Backend to "memory" or "file" in settings.go
```

The default in-memory cache never does this. See [Caching](33-caching.md).

## The sequence

```
coyote makemigrations --name=initial
coyote migrate
coyote createsuperadmin
coyote start
```

`migrate` needs a database it can reach, but not a running server, so the order above works in one
terminal. If the database is unreachable it says so plainly:

```
$ coyote migrate
coyote/cli: no connection to sqlite /path/to/app/coyote.db
```

## createsuperadmin

```
coyote createsuperadmin
coyote createsuperadmin --username=root --email=root@site.com --password=secret
COYOTE_SUPERADMIN_PASSWORD=secret coyote createsuperadmin --username=root
```

Flags win, then the `COYOTE_SUPERADMIN_*` environment variables, then an interactive prompt for
whatever is still missing. The password is **echoed** while you type — use the environment variable
form in scripts and on shared terminals.

The command refuses to run before the `users` table exists:

```
coyote/cli: the users table does not exist yet; run: coyote makemigrations && coyote migrate
```

Users are created through the same service the admin portal uses, so passwords are hashed by one
code path.

## Next

[Deployment →](25-deployment.md)
