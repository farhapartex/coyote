package main

type categoryResource struct{}

func (categoryResource) Entity() any { return Category{} }

func (categoryResource) Label() string { return "Category" }

func (categoryResource) PluralLabel() string { return "Categories" }

func (categoryResource) ListColumns() []string { return []string{"name", "slug", "position"} }

func (categoryResource) DefaultOrder() string { return "position,name" }

func (categoryResource) SearchColumns() []string { return []string{"name", "slug"} }

type productResource struct{}

func (productResource) Entity() any { return Product{} }

func (productResource) Label() string { return "Product" }

func (productResource) ListColumns() []string {
	return []string{"name", "category_id", "price_cents", "stock", "is_active"}
}

func (productResource) DefaultOrder() string { return "name" }

func (productResource) SearchColumns() []string { return []string{"name", "slug", "summary"} }

func (productResource) FilterColumns() []string { return []string{"featured", "is_active"} }
