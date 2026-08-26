app_gofmt() {
	gofmt -w "$EXAMPLE_DIR" 2>/dev/null || true
}

app_write_reference_models() {
	cat >"$EXAMPLE_DIR/models_reference.go" <<'GO'
package main

import "time"

type Customer struct {
	ID        string `gorm:"primaryKey;size:36"`
	Name      string `gorm:"size:200;not null"`
	Code      string `gorm:"uniqueIndex;size:20;not null"`
	Country   string `gorm:"size:2;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}

type Port struct {
	ID        string `gorm:"primaryKey;size:36"`
	Name      string `gorm:"size:200;not null"`
	Code      string `gorm:"uniqueIndex;size:5;not null"`
	Country   string `gorm:"size:2;not null"`
	CreatedAt time.Time
	UpdatedAt time.Time
}
GO
}

app_write_shipment_model() {
	cat >"$EXAMPLE_DIR/models_shipment.go" <<'GO'
package main

import "time"

type Shipment struct {
	ID            string  `gorm:"primaryKey;size:36"`
	Reference     string  `gorm:"size:30;not null;index"`
	CustomerID    *string `gorm:"size:36;index"`
	Customer      Customer
	OriginID      *string `gorm:"size:36;index"`
	Origin        Port    `gorm:"foreignKey:OriginID"`
	DestinationID *string `gorm:"size:36;index"`
	Destination   Port    `gorm:"foreignKey:DestinationID"`
	Status        string  `gorm:"size:20;not null;index"`
	CreatedAt     time.Time
	UpdatedAt     time.Time
}
GO
}

app_write_admin_resources() {
	cat >"$EXAMPLE_DIR/admin_resources.go" <<'GO'
package main

type customerResource struct{}

func (customerResource) Entity() any { return Customer{} }

func (customerResource) ListColumns() []string {
	return []string{"name", "code", "country"}
}

func (customerResource) SearchColumns() []string { return []string{"name", "code"} }

type shipmentResource struct{}

func (shipmentResource) Entity() any { return Shipment{} }

func (shipmentResource) ListColumns() []string {
	return []string{"reference", "customer_id", "origin_id", "destination_id", "status"}
}

func (shipmentResource) SearchColumns() []string { return []string{"reference", "status"} }

type portResource struct{}

func (portResource) Entity() any { return Port{} }

func (portResource) ListColumns() []string {
	return []string{"name", "code", "country"}
}

func (portResource) SearchColumns() []string { return []string{"name", "code"} }
GO
}

app_managed_resources_for_stage() {
	local stage="$1"
	if [ "$stage" -ge 7 ]; then
		printf 'customerResource{}, portResource{}, shipmentResource{}'
	elif [ "$stage" -ge 6 ]; then
		printf 'customerResource{}, portResource{}'
	fi
}

app_registered_models_for_stage() {
	local stage="$1"
	if [ "$stage" -ge 7 ]; then
		printf 'model.Of(Customer{}), model.Of(Port{}), model.Of(Shipment{})'
	elif [ "$stage" -ge 5 ]; then
		printf 'model.Of(Customer{}), model.Of(Port{})'
	fi
}

app_write_main() {
	local stage="$1"
	local models resources
	models="$(app_registered_models_for_stage "$stage")"
	resources="$(app_managed_resources_for_stage "$stage")"

	{
		printf 'package main\n\n'
		printf 'import (\n'
		printf '\t"log"\n'
		printf '\t"net/http"\n\n'
		printf '\t"github.com/farhapartex/coyote/contrib/admin"\n'
		printf '\t"github.com/farhapartex/coyote/core/app"\n'
		if [ -n "$models" ]; then
			printf '\t"github.com/farhapartex/coyote/core/model"\n'
		fi
		printf '\n'
		printf '\t_ "%s/migrations"\n' "$EXAMPLE_NAME"
		printf ')\n\n'
		printf 'func main() {\n'
		printf '\ta := app.New()\n\n'
		if [ -n "$models" ]; then
			printf '\ta.RegisterModel(%s)\n\n' "$models"
		fi
		if [ -n "$resources" ]; then
			printf '\tportal := admin.Mount(a)\n'
			printf '\tportal.MustManage(%s)\n\n' "$resources"
		else
			printf '\tadmin.Mount(a)\n\n'
		fi
		printf '\ta.Get("/{$}", func(w http.ResponseWriter, r *http.Request) {\n'
		printf '\t\ta.Render(w, r, "pages/home.html", app.Data{"Title": "Home"})\n'
		printf '\t}).Named("home")\n\n'
		printf '\tif err := a.Run(); err != nil {\n'
		printf '\t\tlog.Fatal(err)\n'
		printf '\t}\n'
		printf '}\n'
	} >"$EXAMPLE_DIR/main.go"

	app_gofmt
}

app_database_path() {
	printf '%s/%s' "$EXAMPLE_DIR" "$(sed -n 's/^DB_NAME=\(.*\)$/\1/p' "$EXAMPLE_DIR/.env" | head -1)"
}

app_run_command() {
	local command_name="$1"
	shift
	cd "$EXAMPLE_DIR"
	run_capturing_streams "$COYOTE_BIN" "$command_name" "$@"
	cd "$E2E_ROOT"
	APP_OUTPUT="$(printf '%s\n%s' "$CAPTURED_STDOUT" "$CAPTURED_STDERR" |
		grep -v 'level=DEBUG' | grep -v 'level=WARN' | grep -v 'level=INFO' || true)"
}

app_reset_migration_state() {
	rm -f "$EXAMPLE_DIR"/migrations/[0-9][0-9][0-9][0-9]_*.go
	rm -f "$EXAMPLE_DIR/migrations/snapshot.json"
	rm -f "$(app_database_path)" "$(app_database_path)-wal" "$(app_database_path)-shm"
}

app_migration_files() {
	find "$EXAMPLE_DIR/migrations" -name '[0-9][0-9][0-9][0-9]_*.go' -type f 2>/dev/null | sort
}

app_sqlite_query() {
	sqlite3 "$(app_database_path)" "$1" 2>/dev/null || true
}

app_table_exists() {
	[ -n "$(app_sqlite_query "SELECT name FROM sqlite_master WHERE type='table' AND name='$1';")" ]
}

app_table_columns() {
	app_sqlite_query "SELECT name FROM pragma_table_info('$1') ORDER BY name;"
}

app_row_count() {
	app_sqlite_query "SELECT count(*) FROM $1;"
}
