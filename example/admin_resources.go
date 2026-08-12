package main

type productResource struct{}

func (productResource) Entity() any { return Product{} }

func (productResource) ListColumns() []string {
	return []string{"name", "sku", "price", "stock"}
}

type checkoutResource struct{}

func (checkoutResource) Entity() any { return Checkout{} }
