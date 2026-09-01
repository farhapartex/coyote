package auth

import "context"

type SuperadminCounter interface {
	CountActiveSuperadmins(ctx context.Context) (int, error)
}

type guarded struct {
	Store
}

func Guarded(store Store) Store {
	if store == nil {
		return nil
	}
	if _, already := store.(*guarded); already {
		return store
	}
	return &guarded{Store: store}
}

func (g *guarded) Update(ctx context.Context, u *User) error {
	existing, err := g.Store.ByID(ctx, u.ID)
	if err != nil {
		return err
	}
	losing := existing.IsSuperadmin && existing.IsActive && (!u.IsSuperadmin || !u.IsActive || !u.IsStaff)
	if losing {
		remaining, err := g.activeSuperadmins(ctx)
		if err != nil {
			return err
		}
		if remaining < 2 {
			return ErrLastSuperadmin
		}
	}
	return g.Store.Update(ctx, u)
}

func (g *guarded) Delete(ctx context.Context, id string) error {
	existing, err := g.Store.ByID(ctx, id)
	if err != nil {
		return err
	}
	if existing.IsSuperadmin && existing.IsActive {
		remaining, err := g.activeSuperadmins(ctx)
		if err != nil {
			return err
		}
		if remaining < 2 {
			return ErrLastSuperadmin
		}
	}
	return g.Store.Delete(ctx, id)
}

func (g *guarded) activeSuperadmins(ctx context.Context) (int, error) {
	if counter, ok := g.Store.(SuperadminCounter); ok {
		return counter.CountActiveSuperadmins(ctx)
	}
	everyone, err := g.Store.All(ctx)
	if err != nil {
		return 0, err
	}
	total := 0
	for _, u := range everyone {
		if u.IsSuperadmin && u.IsActive {
			total++
		}
	}
	return total, nil
}
