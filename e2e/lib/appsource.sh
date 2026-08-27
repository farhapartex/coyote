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
	local columns="${1:-1}"
	local unique_reference="${2:-0}"
	local reference_size="${3:-30}"

	{
		printf 'package main\n\n'
		printf 'import "time"\n\n'
		printf 'type Shipment struct {\n'
		printf '\tID            string  `gorm:"primaryKey;size:36"`\n'
		if [ "$unique_reference" = "2" ]; then
			printf '\tReference     string  `gorm:"size:%s;not null;uniqueIndex:uq_shipments_reference"`\n' "$reference_size"
		elif [ "$unique_reference" = "1" ]; then
			printf '\tReference     string  `gorm:"size:%s;not null;uniqueIndex"`\n' "$reference_size"
		else
			printf '\tReference     string  `gorm:"size:%s;not null;index"`\n' "$reference_size"
		fi
		printf '\tCustomerID    *string `gorm:"size:36;index"`\n'
		printf '\tCustomer      Customer\n'
		printf '\tOriginID      *string `gorm:"size:36;index"`\n'
		printf '\tOrigin        Port    `gorm:"foreignKey:OriginID"`\n'
		printf '\tDestinationID *string `gorm:"size:36;index"`\n'
		printf '\tDestination   Port    `gorm:"foreignKey:DestinationID"`\n'
		printf '\tStatus        string  `gorm:"size:20;not null;index"`\n'
		if [ "$columns" -ge 2 ]; then
			printf '\tCustomsNotes  *string `gorm:"size:2000"`\n'
			printf '\tDeclaredValue *float64\n'
		fi
		if [ "$columns" -ge 3 ]; then
			printf '\tETA           *time.Time\n'
		fi
		if [ "$columns" -ge 4 ]; then
			printf '\tCarrierName   string `gorm:"size:100;not null"`\n'
		fi
		printf '\tCreatedAt     time.Time\n'
		printf '\tUpdatedAt     time.Time\n'
		printf '}\n'
	} >"$EXAMPLE_DIR/models_shipment.go"

	app_gofmt
}

app_write_container_models() {
	local unique_kind="${1:-0}"
	cat >"$EXAMPLE_DIR/models_container.go" <<'GO'
package main

import "time"

type Container struct {
	ID         string  `gorm:"primaryKey;size:36"`
	Number     string  `gorm:"size:11;not null;uniqueIndex"`
	ShipmentID *string `gorm:"size:36;index"`
	Shipment   Shipment
	SizeFeet   int
	Sealed     bool `gorm:"index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type TrackingEvent struct {
	ID          string  `gorm:"primaryKey;size:36"`
	ShipmentID  *string `gorm:"size:36;index"`
	Shipment    Shipment
	ContainerID *string `gorm:"size:36;index"`
	Container   Container
	Kind        string    `gorm:"size:40;not null;index"`
	Location    string    `gorm:"size:120"`
	OccurredAt  time.Time `gorm:"index"`
	CreatedAt   time.Time
}
GO

	if [ "$unique_kind" = "2" ]; then
		sed -i.bak 's/`gorm:"size:40;not null;index"`/`gorm:"size:40;not null;uniqueIndex:uq_tracking_events_kind"`/' \
			"$EXAMPLE_DIR/models_container.go"
		rm -f "$EXAMPLE_DIR/models_container.go.bak"
	elif [ "$unique_kind" = "1" ]; then
		sed -i.bak 's/`gorm:"size:40;not null;index"`/`gorm:"size:40;not null;uniqueIndex"`/' \
			"$EXAMPLE_DIR/models_container.go"
		rm -f "$EXAMPLE_DIR/models_container.go.bak"
	fi
	app_gofmt
}

app_write_invoice_models() {
	cat >"$EXAMPLE_DIR/models_invoice.go" <<'GO'
package main

import "time"

type Invoice struct {
	ID         string  `gorm:"primaryKey;size:36"`
	Number     string  `gorm:"size:20;not null;uniqueIndex"`
	CustomerID *string `gorm:"size:36;index"`
	Customer   Customer
	Currency   string    `gorm:"size:3;not null"`
	TotalCents int64     `gorm:"not null"`
	Status     string    `gorm:"size:20;not null;index"`
	IssuedAt   time.Time `gorm:"index"`
	CreatedAt  time.Time
	UpdatedAt  time.Time
}

type InvoiceLine struct {
	ID          string  `gorm:"primaryKey;size:36"`
	InvoiceID   *string `gorm:"size:36;index"`
	Invoice     Invoice
	ShipmentID  *string `gorm:"size:36;index"`
	Shipment    Shipment
	Description string `gorm:"size:200;not null"`
	Quantity    int    `gorm:"not null"`
	UnitCents   int64  `gorm:"not null"`
	AmountCents int64  `gorm:"not null"`
	CreatedAt   time.Time
}
GO
	app_gofmt
}

app_write_admin_resources() {
	local stage="${1:-6}"

	{
		printf 'package main\n\n'
		printf 'type customerResource struct{}\n\n'
		printf 'func (customerResource) Entity() any { return Customer{} }\n\n'
		printf 'func (customerResource) ListColumns() []string {\n'
		printf '\treturn []string{"name", "code", "country"}\n}\n\n'
		printf 'func (customerResource) SearchColumns() []string { return []string{"name", "code"} }\n\n'
		printf 'type portResource struct{}\n\n'
		printf 'func (portResource) Entity() any { return Port{} }\n\n'
		printf 'func (portResource) ListColumns() []string {\n'
		printf '\treturn []string{"name", "code", "country"}\n}\n\n'
		printf 'func (portResource) SearchColumns() []string { return []string{"name", "code"} }\n'

		if [ "$stage" -ge 7 ]; then
			printf '\ntype shipmentResource struct{}\n\n'
			printf 'func (shipmentResource) Entity() any { return Shipment{} }\n\n'
			printf 'func (shipmentResource) ListColumns() []string {\n'
			printf '\treturn []string{"reference", "customer_id", "origin_id", "destination_id", "status"}\n}\n\n'
			printf 'func (shipmentResource) SearchColumns() []string { return []string{"reference", "status"} }\n'
		fi

		if [ "$stage" -ge 9 ]; then
			printf '\ntype containerResource struct{}\n\n'
			printf 'func (containerResource) Entity() any { return Container{} }\n\n'
			printf 'func (containerResource) ListColumns() []string {\n'
			printf '\treturn []string{"number", "shipment_id", "size_feet", "sealed"}\n}\n\n'
			printf 'func (containerResource) SearchColumns() []string { return []string{"number"} }\n\n'
			printf 'type trackingEventResource struct{}\n\n'
			printf 'func (trackingEventResource) Entity() any { return TrackingEvent{} }\n\n'
			printf 'func (trackingEventResource) ListColumns() []string {\n'
			printf '\treturn []string{"kind", "shipment_id", "container_id", "location", "occurred_at"}\n}\n\n'
			printf 'func (trackingEventResource) SearchColumns() []string { return []string{"kind", "location"} }\n'
		fi

		if [ "$stage" -ge 12 ]; then
			printf '\ntype invoiceResource struct{}\n\n'
			printf 'func (invoiceResource) Entity() any { return Invoice{} }\n\n'
			printf 'func (invoiceResource) ListColumns() []string {\n'
			printf '\treturn []string{"number", "customer_id", "currency", "total_cents", "status"}\n}\n\n'
			printf 'func (invoiceResource) SearchColumns() []string { return []string{"number", "status"} }\n\n'
			printf 'type invoiceLineResource struct{}\n\n'
			printf 'func (invoiceLineResource) Entity() any { return InvoiceLine{} }\n\n'
			printf 'func (invoiceLineResource) ListColumns() []string {\n'
			printf '\treturn []string{"description", "invoice_id", "shipment_id", "quantity", "amount_cents"}\n}\n\n'
			printf 'func (invoiceLineResource) SearchColumns() []string { return []string{"description"} }\n'
		fi
	} >"$EXAMPLE_DIR/admin_resources.go"

	app_gofmt
}

app_managed_resources_for_stage() {
	local stage="$1"
	if [ "$stage" -ge 12 ]; then
		printf 'customerResource{}, portResource{}, shipmentResource{}, containerResource{}, trackingEventResource{}, invoiceResource{}, invoiceLineResource{}'
	elif [ "$stage" -ge 9 ]; then
		printf 'customerResource{}, portResource{}, shipmentResource{}, containerResource{}, trackingEventResource{}'
	elif [ "$stage" -ge 7 ]; then
		printf 'customerResource{}, portResource{}, shipmentResource{}'
	elif [ "$stage" -ge 6 ]; then
		printf 'customerResource{}, portResource{}'
	fi
}

app_registered_models_for_stage() {
	local stage="$1"
	if [ "$stage" -ge 12 ]; then
		printf 'model.Of(Customer{}), model.Of(Port{}), model.Of(Shipment{}), model.Of(Container{}), model.Of(TrackingEvent{}), model.Of(Invoice{}), model.Of(InvoiceLine{})'
	elif [ "$stage" -ge 9 ]; then
		printf 'model.Of(Customer{}), model.Of(Port{}), model.Of(Shipment{}), model.Of(Container{}), model.Of(TrackingEvent{})'
	elif [ "$stage" -ge 7 ]; then
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

app_snapshot_path() {
	printf '%s/migrations/snapshot.json' "$EXAMPLE_DIR"
}

app_backup_snapshot() {
	cp "$(app_snapshot_path)" "$E2E_WORK_DIR/snapshot.backup" 2>/dev/null || true
}

app_restore_snapshot() {
	if [ -f "$E2E_WORK_DIR/snapshot.backup" ]; then
		cp "$E2E_WORK_DIR/snapshot.backup" "$(app_snapshot_path)"
		rm -f "$E2E_WORK_DIR/snapshot.backup"
	fi
}

app_add_missing_model_import() {
	local file="$1"
	if grep -q 'coyote/core/model' "$file" 2>/dev/null; then
		return 1
	fi
	sed -i.bak 's|^import (|import (\
	"github.com/farhapartex/coyote/core/model"\
|' "$file"
	rm -f "$file.bak"
	app_gofmt
	return 0
}

app_reset_migration_state() {
	rm -f "$EXAMPLE_DIR"/migrations/[0-9][0-9][0-9][0-9]_*.go
	rm -f "$EXAMPLE_DIR/migrations/snapshot.json"
	rm -f "$(app_database_path)" "$(app_database_path)-wal" "$(app_database_path)-shm"
}

app_migration_for_name() {
	find "$EXAMPLE_DIR/migrations" -name "[0-9][0-9][0-9][0-9]_$1.go" -type f 2>/dev/null | head -1
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
