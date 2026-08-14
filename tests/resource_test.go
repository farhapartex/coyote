package tests

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"github.com/farhapartex/coyote/admin"
	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/repo"
	"github.com/farhapartex/coyote/core/settings"
)

type Product struct {
	ID          string `gorm:"primaryKey;size:64"`
	Name        string `gorm:"size:200;not null"`
	SKU         string `gorm:"uniqueIndex;size:64;not null"`
	Price       float64
	Stock       int
	IsPublished bool
	ReleasedAt  *time.Time
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type productResource struct{}

func (productResource) Entity() any { return Product{} }

type narrowProduct struct{}

func (narrowProduct) Entity() any             { return Product{} }
func (narrowProduct) ListColumns() []string   { return []string{"name", "sku"} }
func (narrowProduct) HiddenColumns() []string { return []string{"stock"} }
func (narrowProduct) Label() string           { return "Item" }
func (narrowProduct) PluralLabel() string     { return "Items" }
func (narrowProduct) Slug() string            { return "items" }

type frozenProduct struct{ productResource }

func (frozenProduct) ReadOnly() bool { return true }
func (frozenProduct) Slug() string   { return "frozen" }

type reservedResource struct{ productResource }

func (reservedResource) Slug() string { return "users" }

func migratedApp(t *testing.T, entities ...any) *app.App {
	t.Helper()
	a := newTestApp(t, func(s *settings.Settings) { s.Logging.Level = "error" })
	models := make([]model.Model, 0, len(entities))
	for _, entity := range entities {
		models = append(models, model.Of(entity))
	}
	a.RegisterModel(models...)
	syncSchema(t, a)
	return a
}

func TestDescribeDerivesFieldsFromEntity(t *testing.T) {
	a := migratedApp(t, Product{})
	schema, err := a.Describe(Product{})
	if err != nil {
		t.Fatal(err)
	}

	if schema.Table != "products" || schema.Slug != "products" {
		t.Errorf("table/slug = %q/%q", schema.Table, schema.Slug)
	}
	if schema.Plural != "Products" || schema.Label != "Product" {
		t.Errorf("labels = %q/%q", schema.Plural, schema.Label)
	}
	if schema.Key.Column != "id" || !schema.Key.PrimaryKey {
		t.Errorf("key = %+v", schema.Key)
	}

	kinds := map[string]model.Kind{}
	required := map[string]bool{}
	for _, f := range schema.Fields {
		kinds[f.Column] = f.Kind
		required[f.Column] = f.Required
	}
	for column, want := range map[string]model.Kind{
		"name": model.KindString, "price": model.KindFloat,
		"stock": model.KindInt, "is_published": model.KindBool,
		"released_at": model.KindTime,
	} {
		if kinds[column] != want {
			t.Errorf("%s kind = %q, want %q", column, kinds[column], want)
		}
	}
	if !required["name"] || !required["sku"] {
		t.Error("not-null columns should be required")
	}
	if required["price"] || required["stock"] || required["released_at"] {
		t.Error("nullable columns should be optional")
	}
}

func TestPrimaryKeyAndTimestampsAreNotEditable(t *testing.T) {
	a := migratedApp(t, Product{})
	schema, _ := a.Describe(Product{})

	for _, f := range schema.FormFields() {
		if f.PrimaryKey {
			t.Error("the primary key must never be an editable field")
		}
		if f.Column == "created_at" || f.Column == "updated_at" {
			t.Errorf("%s is framework managed and must not be editable", f.Column)
		}
	}
	display := schema.DisplayFields(true)
	if len(display) == 0 || display[0].Column != "id" {
		t.Error("DisplayFields should lead with the key for existing records")
	}
	if got := schema.DisplayFields(false); len(got) > 0 && got[0].Column == "id" {
		t.Error("DisplayFields must omit the key when creating")
	}
}

func TestListColumnsCapAndOverrides(t *testing.T) {
	a := migratedApp(t, Product{})
	schema, _ := a.Describe(Product{})
	if len(schema.ListFields()) > 4 {
		t.Errorf("list should cap at 4 columns, got %d", len(schema.ListFields()))
	}

	narrowed, _ := a.Describe(Product{})
	narrowed.SetListColumns([]string{"name", "sku"})
	if len(narrowed.ListFields()) != 2 {
		t.Errorf("override ignored: %d columns", len(narrowed.ListFields()))
	}

	hidden, _ := a.Describe(Product{})
	hidden.Hide([]string{"stock"})
	for _, f := range hidden.Visible() {
		if f.Column == "stock" {
			t.Error("hidden column still visible")
		}
	}
}

func TestStoreCRUDRoundTrip(t *testing.T) {
	a := migratedApp(t, Product{})
	schema, _ := a.Describe(Product{})
	store, err := a.Store()
	if err != nil {
		t.Fatal(err)
	}
	ctx := context.Background()

	id, err := store.Insert(ctx, schema, model.Record{
		"name": "Desert Boot", "sku": "DB-1", "price": 89.95, "stock": int64(12), "is_published": true,
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(id) != 36 || strings.Count(id, "-") != 4 {
		t.Errorf("expected a generated uuid, got %q", id)
	}

	found, err := store.Find(ctx, schema, id)
	if err != nil {
		t.Fatal(err)
	}
	if found.String("name") != "Desert Boot" || found.String("sku") != "DB-1" {
		t.Errorf("unexpected record: %+v", found)
	}
	if !found.Bool("is_published") {
		t.Error("bool did not round trip")
	}

	if err := store.Update(ctx, schema, id, model.Record{"name": "Desert Boot II", "stock": int64(3)}); err != nil {
		t.Fatal(err)
	}
	found, _ = store.Find(ctx, schema, id)
	if found.String("name") != "Desert Boot II" {
		t.Errorf("update did not apply: %+v", found)
	}
	if found.String("id") != id {
		t.Error("the key changed on update")
	}

	page, err := store.List(ctx, schema, model.Query{Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 1 || len(page.Records) != 1 {
		t.Errorf("list = %d records, total %d", len(page.Records), page.Total)
	}

	if err := store.Delete(ctx, schema, id); err != nil {
		t.Fatal(err)
	}
	if _, err := store.Find(ctx, schema, id); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("got %v, want ErrNotFound", err)
	}
	if err := store.Update(ctx, schema, id, model.Record{"name": "x"}); !errors.Is(err, repo.ErrNotFound) {
		t.Errorf("updating a missing record: got %v, want ErrNotFound", err)
	}
}

func adminWithResources(t *testing.T, resources ...admin.Resource) (*app.App, *client, *admin.Admin) {
	t.Helper()
	a := migratedApp(t, Product{})
	if _, err := a.Auth.CreateSuperadmin("root", "root@example.com", "supersecret"); err != nil {
		t.Fatal(err)
	}
	portal := admin.Mount(a)
	if err := portal.Manage(resources...); err != nil {
		t.Fatalf("managing resources: %v", err)
	}
	c := newClient(t, a.Handler())
	c.login("/admin/login", "root", "supersecret")
	return a, c, portal
}

func TestDynamicRoutesExistWithoutManualRegistration(t *testing.T) {
	_, c, portal := adminWithResources(t, productResource{})

	if got := portal.Managed(); len(got) != 1 || got[0] != "products" {
		t.Fatalf("managed = %v", got)
	}
	for _, path := range []string{"/admin/products", "/admin/products/new"} {
		if rec := c.get(path); rec.Code != http.StatusOK {
			t.Errorf("%s = %d, want 200", path, rec.Code)
		}
	}
	if rec := c.get("/admin/unregistered"); rec.Code != http.StatusNotFound {
		t.Errorf("unregistered resource = %d, want 404", rec.Code)
	}

	body := c.get("/admin/").Body.String()
	if !strings.Contains(body, `href="/admin/products"`) {
		t.Error("the sidebar should link to managed resources")
	}
}

func TestDynamicFormReflectsColumnTypes(t *testing.T) {
	_, c, _ := adminWithResources(t, productResource{})
	body := c.get("/admin/products/new").Body.String()

	for _, want := range []string{
		`type="text" name="name"`,
		`type="text" name="sku"`,
		`type="number" name="price"`,
		`type="number" name="stock"`,
		`type="checkbox" name="is_published"`,
		`type="datetime-local" name="released_at"`,
	} {
		if !strings.Contains(body, want) {
			t.Errorf("form missing %s", want)
		}
	}
	if strings.Contains(body, `name="id"`) {
		t.Error("the create form must not ask for the primary key")
	}
	if strings.Contains(body, `name="created_at"`) {
		t.Error("framework timestamps must not be editable")
	}
	if strings.Count(body, "required") < 2 {
		t.Error("not-null columns should render as required")
	}
}

func TestDynamicCRUDOverHTTP(t *testing.T) {
	a, c, _ := adminWithResources(t, productResource{})

	token := c.token("/admin/products/new")
	rec := c.do(http.MethodPost, "/admin/products/new", url.Values{
		"csrf_token": {token}, "name": {"Desert Boot"}, "sku": {"DB-1"},
		"price": {"89.95"}, "stock": {"12"}, "is_published": {"1"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("create = %d, want 303: %s", rec.Code, rec.Body.String())
	}

	schema, _ := a.Describe(Product{})
	store, _ := a.Store()
	page, _ := store.List(context.Background(), schema, model.Query{Limit: 5})
	if page.Total != 1 {
		t.Fatalf("expected 1 row, got %d", page.Total)
	}
	id := page.Records[0].String("id")

	list := c.get("/admin/products").Body.String()
	if !strings.Contains(list, "Desert Boot") || !strings.Contains(list, "Details") {
		t.Error("list should show the record and an action column")
	}

	form := c.get("/admin/products/" + id).Body.String()
	if !strings.Contains(form, `name="id"`) || !strings.Contains(form, "readonly") {
		t.Error("the detail form should show the key read only")
	}

	token = c.token("/admin/products/" + id)
	rec = c.do(http.MethodPost, "/admin/products/"+id, url.Values{
		"csrf_token": {token}, "id": {"tampered"}, "name": {"Desert Boot II"},
		"sku": {"DB-1"}, "price": {"99"}, "stock": {"5"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("update = %d, want 303", rec.Code)
	}
	updated, err := store.Find(context.Background(), schema, id)
	if err != nil {
		t.Fatalf("the key was changed by a tampered form: %v", err)
	}
	if updated.String("name") != "Desert Boot II" {
		t.Errorf("update did not apply: %+v", updated)
	}
	if updated.Bool("is_published") {
		t.Error("an unchecked box should clear the flag")
	}

	token = c.token("/admin/products/" + id)
	rec = c.do(http.MethodPost, "/admin/products/"+id+"/delete", url.Values{"csrf_token": {token}})
	if rec.Code != http.StatusSeeOther {
		t.Fatalf("delete = %d, want 303", rec.Code)
	}
	if _, err := store.Find(context.Background(), schema, id); !errors.Is(err, repo.ErrNotFound) {
		t.Error("record should be deleted")
	}
}

func TestDynamicFormValidation(t *testing.T) {
	a, c, _ := adminWithResources(t, productResource{})

	token := c.token("/admin/products/new")
	rec := c.do(http.MethodPost, "/admin/products/new", url.Values{
		"csrf_token": {token}, "sku": {"DB-2"}, "price": {"1"},
	})
	if rec.Code != http.StatusBadRequest {
		t.Errorf("missing required field = %d, want 400", rec.Code)
	}
	if !strings.Contains(rec.Body.String(), "Name is required") {
		t.Errorf("expected a required-field message:\n%s", rec.Body.String())
	}

	token = c.token("/admin/products/new")
	rec = c.do(http.MethodPost, "/admin/products/new", url.Values{
		"csrf_token": {token}, "name": {"X"}, "sku": {"DB-3"}, "price": {"not a number"},
	})
	if !strings.Contains(rec.Body.String(), "must be a number") {
		t.Error("expected a type error for a bad number")
	}

	schema, _ := a.Describe(Product{})
	store, _ := a.Store()
	page, _ := store.List(context.Background(), schema, model.Query{Limit: 5})
	if page.Total != 0 {
		t.Errorf("invalid submissions must not persist, found %d rows", page.Total)
	}
}

func TestResourceInterfaceOverrides(t *testing.T) {
	_, c, portal := adminWithResources(t, narrowProduct{})

	if got := portal.Managed(); len(got) != 1 || got[0] != "items" {
		t.Fatalf("slug override ignored: %v", got)
	}
	if rec := c.get("/admin/products"); rec.Code != http.StatusNotFound {
		t.Error("the default slug should not be routed after an override")
	}

	body := c.get("/admin/items").Body.String()
	if !strings.Contains(body, "Items") {
		t.Error("plural label override ignored")
	}
	if strings.Contains(c.get("/admin/items/new").Body.String(), `name="stock"`) {
		t.Error("hidden column should not appear in the form")
	}
}

func TestReadOnlyResourceRejectsWrites(t *testing.T) {
	a, c, _ := adminWithResources(t, frozenProduct{})

	body := c.get("/admin/frozen").Body.String()
	if strings.Contains(body, "/admin/frozen/new") {
		t.Error("a read only resource should not offer an add button")
	}

	token := c.token("/admin/frozen/new")
	rec := c.do(http.MethodPost, "/admin/frozen/new", url.Values{
		"csrf_token": {token}, "name": {"Nope"}, "sku": {"N-1"},
	})
	if rec.Code != http.StatusSeeOther {
		t.Errorf("write to read only resource = %d, want a redirect", rec.Code)
	}

	schema, _ := a.Describe(Product{})
	store, _ := a.Store()
	page, _ := store.List(context.Background(), schema, model.Query{Limit: 5})
	if page.Total != 0 {
		t.Error("a read only resource must not accept writes")
	}
}

func TestReservedAndDuplicateSlugsAreRejected(t *testing.T) {
	a := migratedApp(t, Product{})
	portal := admin.Mount(a)

	if err := portal.Manage(reservedResource{}); !errors.Is(err, admin.ErrReservedSlug) {
		t.Errorf("got %v, want ErrReservedSlug", err)
	}
	if err := portal.Manage(productResource{}); err != nil {
		t.Fatal(err)
	}
	if err := portal.Manage(productResource{}); !errors.Is(err, admin.ErrDuplicateSlug) {
		t.Errorf("got %v, want ErrDuplicateSlug", err)
	}
}

func TestManageBeforeMountIsRejected(t *testing.T) {
	if err := (&admin.Admin{}).Manage(productResource{}); !errors.Is(err, admin.ErrNotMountedYet) {
		t.Errorf("got %v, want ErrNotMountedYet", err)
	}
}

func TestFieldLabels(t *testing.T) {
	a := migratedApp(t, Product{})
	schema, _ := a.Describe(Product{})

	labels := map[string]string{}
	for _, f := range schema.Fields {
		labels[f.Column] = f.Label
	}
	for column, want := range map[string]string{
		"id": "ID", "sku": "SKU", "is_published": "Is published",
		"created_at": "Created", "released_at": "Released",
	} {
		if labels[column] != want {
			t.Errorf("label for %q = %q, want %q", column, labels[column], want)
		}
	}
}
