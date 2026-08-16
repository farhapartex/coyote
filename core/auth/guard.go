package auth

type SuperadminCounter interface {
	CountActiveSuperadmins() (int, error)
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

func (g *guarded) Update(u *User) error {
	existing, err := g.Store.ByID(u.ID)
	if err != nil {
		return err
	}
	losing := existing.IsSuperadmin && existing.IsActive && (!u.IsSuperadmin || !u.IsActive || !u.IsStaff)
	if losing {
		remaining, err := g.activeSuperadmins()
		if err != nil {
			return err
		}
		if remaining < 2 {
			return ErrLastSuperadmin
		}
	}
	return g.Store.Update(u)
}

func (g *guarded) Delete(id string) error {
	existing, err := g.Store.ByID(id)
	if err != nil {
		return err
	}
	if existing.IsSuperadmin && existing.IsActive {
		remaining, err := g.activeSuperadmins()
		if err != nil {
			return err
		}
		if remaining < 2 {
			return ErrLastSuperadmin
		}
	}
	return g.Store.Delete(id)
}

func (g *guarded) activeSuperadmins() (int, error) {
	if counter, ok := g.Store.(SuperadminCounter); ok {
		return counter.CountActiveSuperadmins()
	}
	total := 0
	for _, u := range g.Store.All() {
		if u.IsSuperadmin && u.IsActive {
			total++
		}
	}
	return total, nil
}
