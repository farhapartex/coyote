# Permissions and roles

[← Back to contents](README.md)

Four permissions per model, bundled into roles, granted to staff. Superadmins bypass the whole
system.

## The shape of it

```
model  ──sync──▶  4 permissions  ──▶  role  ──▶  user
                  create/read/                    (staff)
                  update/delete
```

| Table | What it holds |
| --- | --- |
| `permissions` | one row per resource and action, e.g. `products.create` |
| `roles` | a named bundle, e.g. "Catalogue editor" |
| `role_permissions` | which permissions a role carries |
| `user_roles` | which roles a user holds |

**No roles are created for you.** Permissions exist as soon as you sync; who gets them is entirely
your decision.

## Turning it off

On by default. To skip the four tables entirely:

```go
s.Auth.Permissions = false
```

With permissions off, every staff account can reach every managed resource, and the Roles section
disappears from the portal.

## Creating the permissions

Permissions are derived from your registered models, so they are created by a command rather than
guessed at runtime:

```
coyote syncpermissions
```

`coyote migrate` runs the same sync after it applies, so in practice you rarely call it directly.

```
$ coyote syncpermissions
  + products.create
  + products.delete
  + products.read
  + products.update

created 4 permission(s)
```

Syncing is idempotent — run it as often as you like. A model that has disappeared is **reported,
never deleted**:

```
2 permission(s) no longer match a model; review before removing them:
  ! widgets.read
  ! widgets.update
```

Deleting them automatically would silently revoke whatever roles still grant them, so that is left
to you.

Join tables and the session table are skipped; they are not things anyone browses.

## Codenames

`resource.action`, where the resource is the **table name**:

```go
auth.Codename("products", auth.ActionCreate)   // "products.create"
auth.ActionCreate, auth.ActionRead, auth.ActionUpdate, auth.ActionDelete
```

If you renamed the table with `model.Named`, the codename follows the new name.

## Checking a permission

```go
if a.Auth.Can(r, "products.update") {
	...
}

a.Auth.CanAny(r, "products.update", "products.delete")
```

As middleware, on a route or a group:

```go
a.Post("/products/{id}", update, a.Auth.RequirePermission("products.update"))

catalogue := a.Group("/catalogue", a.Auth.RequirePermission("products.read"))
```

Anonymous requests get 401; a signed-in user without the permission gets 403.

Rules that always hold:

- **A superadmin passes every check**, without a query.
- An inactive account passes nothing.
- Permissions come only through roles; there are no per-user grants.
- The resolved set is cached per request, so a handler that checks ten permissions still costs one
  query.

## Assigning them

From the admin portal — **Roles** in the sidebar, superadmin only. Create a role, tick the
permissions in the grid, then assign the role on the user's own form.

From code, when you are seeding an environment:

```go
store := a.Auth.Permissions()

role := &auth.Role{Name: "Catalogue editor"}
store.CreateRole(role)

all, _ := store.AllPermissions()
ids := []string{}
for _, permission := range all {
	if permission.Resource == "products" && permission.Action != auth.ActionDelete {
		ids = append(ids, permission.ID)
	}
}
store.SetRolePermissions(role.ID, ids)
store.SetUserRoles(user.ID, []string{role.ID})
```

`SetRolePermissions` and `SetUserRoles` **replace** what was there, so they are safe to call
repeatedly from a seed script.

Deleting a role removes its grants and its assignments in one transaction — nobody is left holding
a permission through a role that no longer exists.

## In the admin portal

Once permissions are on, managed resources are gated automatically:

| Route | Needs |
| --- | --- |
| `GET /admin/products` | `products.read` |
| `GET/POST /admin/products/new` | `products.create` |
| `POST /admin/products/{id}` | `products.update` |
| `POST /admin/products/{id}/delete` | `products.delete` |

The sidebar only lists resources the signed-in user can read, so staff are not shown links that
would refuse them.

Users, sessions and roles remain **superadmin-only** regardless of permissions.

## Your own backend

`auth.PermissionStore` is an interface. Implement it and set `s.Auth.PermissionStore` to keep
permissions somewhere other than the default tables.

## Next

- [Authentication →](14-authentication.md) — the staff tier these permissions sit on
- [Admin portal →](15-admin.md)
