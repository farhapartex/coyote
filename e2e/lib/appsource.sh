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
	local with_document="${4:-0}"

	{
		printf 'package main\n\n'
		if [ "$with_document" = "1" ]; then
			printf 'import (\n\t"time"\n\n\t"github.com/farhapartex/coyote/core/upload"\n)\n\n'
		else
			printf 'import "time"\n\n'
		fi
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
		if [ "$with_document" = "1" ]; then
			printf '\tDocument      upload.Ref `gorm:"size:200" coyote:"path=shipments/documents,accept=application/pdf"`\n'
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

app_write_settings() {
	local uploads="${1:-0}"
	local private="${2:-0}"
	local per_page="${3:-0}"
	local caches="${4:-0}"
	local page_ttl="${5:-3}"
	local locales="${6:-0}"
	local email="${7:-0}"

	{
		printf 'package main\n\n'
		printf 'import (\n'
		printf '\t"embed"\n'
		printf '\t"io/fs"\n'
		if [ "$caches" = "1" ]; then
			printf '\t"time"\n'
		fi
		printf '\n'
		printf '\t"github.com/farhapartex/coyote/core/settings"\n'
		printf ')\n\n'
		printf '//go:embed templates\n'
		printf 'var templateFS embed.FS\n\n'
		printf '//go:embed static\n'
		printf 'var staticFS embed.FS\n\n'
		if [ "$locales" = "1" ]; then
			printf '//go:embed locales\n'
			printf 'var localeFS embed.FS\n\n'
		fi
		printf 'func init() {\n'
		printf '\ttemplates, _ := fs.Sub(templateFS, "templates")\n'
		printf '\tstatic, _ := fs.Sub(staticFS, "static")\n'
		if [ "$locales" = "1" ]; then
			printf '\tlocales, _ := fs.Sub(localeFS, "locales")\n'
		fi
		printf '\n'
		printf '\tsettings.MustLoadDotEnv(".env")\n\n'
		printf '\tsettings.Configure(\n'
		printf '\t\tsettings.Preset(settings.Env("APP_ENV", "development")),\n'
		printf '\t\tfunc(s *settings.Settings) {\n'
		printf '\t\t\ts.SecretKey = settings.Env("SECRET_KEY", "")\n'
		printf '\t\t\ts.AllowedHosts = settings.EnvList("ALLOWED_HOSTS", []string{"127.0.0.1", "localhost"})\n\n'
		printf '\t\t\ts.Server.Host = settings.Env("HOST", "127.0.0.1")\n'
		printf '\t\t\ts.Server.Port = settings.EnvInt("PORT", 8000)\n\n'
		printf '\t\t\ts.Databases = []settings.Database{\n'
		printf '\t\t\t\tsettings.SQLiteDatabase("default", settings.Env("DB_NAME", "example.db")),\n'
		printf '\t\t\t}\n\n'
		if [ "$uploads" = "1" ]; then
			printf '\t\t\ts.Uploads.Enabled = true\n'
			printf '\t\t\ts.Uploads.Dir = "media"\n'
			printf '\t\t\ts.Uploads.MaxSize = 1 << 20\n'
			printf '\t\t\ts.Uploads.Allowed = []string{"application/pdf", "image/png", "image/jpeg"}\n'
			printf '\t\t\ts.Uploads.Serve = true\n'
			printf '\t\t\ts.Uploads.URL = "/media/"\n'
			if [ "$private" = "1" ]; then
				printf '\t\t\ts.Uploads.Private = true\n'
			fi
			printf '\n'
		fi
		printf '\t\t\ts.Templates.FS = templates\n'
		printf '\t\t\ts.Static.FS = static\n\n'
		if [ "$caches" = "1" ]; then
			printf '\t\t\ts.Caches = []settings.Cache{\n'
			printf '\t\t\t\tsettings.MemoryCache("default"),\n'
			printf '\t\t\t\t{Alias: "pages", Backend: settings.CacheInFile, Dir: "cache/pages", TTL: time.Hour},\n'
			printf '\t\t\t}\n\n'
			printf '\t\t\ts.PageCache = settings.PageCache{\n'
			printf '\t\t\t\tEnabled: true,\n'
			printf '\t\t\t\tAlias:   "pages",\n'
			printf '\t\t\t\tTTL:     %s * time.Second,\n' "$page_ttl"
			printf '\t\t\t\tPaths:   []string{"/about", "/quote", "/report"},\n'
			printf '\t\t\t}\n\n'
		fi
		if [ "$caches" = "redis" ]; then
			printf '\t\t\ts.Caches = []settings.Cache{\n'
			printf '\t\t\t\tsettings.RedisCache("default", "127.0.0.1:6399"),\n'
			printf '\t\t\t}\n\n'
		fi
		if [ "$per_page" != "0" ]; then
			printf '\t\t\ts.Pagination.PerPage = %s\n\n' "$per_page"
		if [ "$locales" = "1" ]; then
			printf '\t\t\ts.I18N = settings.I18N{\n'
			printf '\t\t\t\tDefault:   "en",\n'
			printf '\t\t\t\tSupported: []string{"en", "fr", "ar"},\n'
			printf '\t\t\t\tFS:        locales,\n'
			printf '\t\t\t}\n\n'
		fi
		if [ "$email" = "1" ]; then
			printf '\t\t\ts.Email = settings.Email{\n'
			printf '\t\t\t\tBackend: settings.EmailToFile,\n'
			printf '\t\t\t\tDir:     "mail",\n'
			printf '\t\t\t\tFrom:    "quotes@meridian.test",\n'
			printf '\t\t\t}\n\n'
		fi
		fi
		printf '\t\t\ts.Admin.SiteName = "Example administration"\n'
		printf '\t\t},\n'
		printf '\t)\n'
		printf '}\n'
	} >"$EXAMPLE_DIR/settings.go"

	app_gofmt
}

app_write_locales() {
	local dir="$EXAMPLE_DIR/locales"
	mkdir -p "$dir"

	cat >"$dir/fr.po" <<'PO'
msgid ""
msgstr ""
"Language: fr\n"
"MIME-Version: 1.0\n"
"Content-Type: text/plain; charset=UTF-8\n"
"Content-Transfer-Encoding: 8bit\n"
"Plural-Forms: nplurals=2; plural=(n > 1);\n"

msgid "Services"
msgstr "Services proposés"

msgid "Track a shipment"
msgstr "Suivre un envoi"

msgid "Request a quote"
msgstr "Demander un tarif"

msgid "About Meridian"
msgstr "À propos de Meridian"

msgid "%d shipment on this lane"
msgid_plural "%d shipments on this lane"
msgstr[0] "%d envoi sur cette ligne"
msgstr[1] "%d envois sur cette ligne"
PO

	cat >"$dir/ar.po" <<'PO'
msgid ""
msgstr ""
"Language: ar\n"
"MIME-Version: 1.0\n"
"Content-Type: text/plain; charset=UTF-8\n"
"Content-Transfer-Encoding: 8bit\n"
"Plural-Forms: nplurals=6; plural=(n==0 ? 0 : n==1 ? 1 : n==2 ? 2 : n%100>=3 && n%100<=10 ? 3 : n%100>=11 ? 4 : 5);\n"

msgid "Services"
msgstr "الخدمات"

msgid "Track a shipment"
msgstr "تتبع الشحنة"

msgid "Request a quote"
msgstr "اطلب عرض سعر"

msgid "About Meridian"
msgstr "عن ميريديان"

msgid "%d shipment on this lane"
msgid_plural "%d shipments on this lane"
msgstr[0] "لا شحنات على هذا الخط"
msgstr[1] "شحنة واحدة على هذا الخط"
msgstr[2] "شحنتان على هذا الخط"
msgstr[3] "%d شحنات على هذا الخط"
msgstr[4] "%d شحنة على هذا الخط"
msgstr[5] "%d شحنة على هذا الخط"
PO
}

app_write_report_handler() {
	cat >"$EXAMPLE_DIR/handlers_report.go" <<'GO'
package main

import (
	"context"
	"net/http"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/cache"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/view"
)

type laneRow struct {
	Status string
	Count  int64
}

type laneReport struct {
	Rows  []laneRow
	Built time.Time
	Runs  int
}

var reportRuns = 0

func buildLaneReport(ctx context.Context, a *app.App) (laneReport, error) {
	records, err := a.Store()
	if err != nil {
		return laneReport{}, err
	}
	schema, err := a.Describe(Shipment{})
	if err != nil {
		return laneReport{}, err
	}

	report := laneReport{Built: time.Now()}
	for _, status := range []string{"booked", "in_transit", "draft"} {
		total, err := records.Count(ctx, schema, model.Query{
			Filters: []model.Filter{{Column: "status", Op: model.Eq, Value: status}},
		})
		if err != nil {
			return laneReport{}, err
		}
		report.Rows = append(report.Rows, laneRow{Status: status, Count: total})
	}

	reportRuns++
	report.Runs = reportRuns
	return report, nil
}

func laneReportPage(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		report, err := cache.Remember(r.Context(), a.Cache(), "reports:lanes", 30*time.Second,
			func(ctx context.Context) (laneReport, error) {
				return buildLaneReport(ctx, a)
			})
		if err != nil {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
			return
		}

		a.Render(w, r, "pages/report.html", view.Data{
			"Title":  "Lane report",
			"Report": report,
			"Age":    time.Since(report.Built).Round(time.Second).String(),
		})
	}
}
GO
	app_gofmt
}

app_write_shipment_list_handler() {
	cat >"$EXAMPLE_DIR/handlers_list.go" <<'GO'
package main

import (
	"net/http"
	"strconv"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/view"
)

func shipmentList(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		records, err := a.Store()
		if err != nil {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
			return
		}
		schema, err := a.Describe(Shipment{})
		if err != nil {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
			return
		}

		total, err := records.Count(r.Context(), schema, model.Query{})
		if err != nil {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
			return
		}

		number, _ := strconv.Atoi(r.URL.Query().Get("page"))
		page := view.Paginate(a.Settings.Pagination.Paginator, total, number,
			a.Settings.Pagination.PerPage)

		query := model.Query{
			Limit:  page.Limit,
			Offset: page.Offset,
			Order:  "reference",
			Sort:   r.URL.Query().Get("sort"),
			With:   []string{"origin_id", "destination_id"},
		}

		result, err := records.List(r.Context(), schema, query)
		if err != nil {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
			return
		}

		a.Render(w, r, "pages/shipments.html", view.Data{
			"Title":     "Shipments",
			"Shipments": result.Records,
			"Page":      page,
			"Sort":      r.URL.Query().Get("sort"),
		})
	}
}
GO
	app_gofmt
}

app_write_quote_model() {
	cat >"$EXAMPLE_DIR/models_quote.go" <<'GO'
package main

import "time"

type QuoteRequest struct {
	ID          string `gorm:"primaryKey;size:36"`
	Company     string `gorm:"size:120;not null"`
	Email       string `gorm:"size:320;not null;index"`
	OriginCode  string `gorm:"size:5;not null"`
	Service     string `gorm:"size:30;not null;index"`
	Containers  int    `gorm:"not null"`
	Notes       string `gorm:"size:2000"`
	SubmittedAt time.Time
	CreatedAt   time.Time
}
GO
	app_gofmt
}

app_write_quote_handlers() {
	local with_email="${1:-0}"
	cat >"$EXAMPLE_DIR/handlers_quote.go" <<'GO'
package main

import (
	"net/http"
	"strings"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/form"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/view"
)

type quoteForm struct {
	Company    string `form:"company" validate:"required,max=120"`
	Email      string `form:"email" validate:"required,email"`
	OriginCode string `form:"origin_code" validate:"required,len=5,alphanum"`
	Service    string `form:"service" validate:"required,oneof=ocean|air|customs"`
	Containers int    `form:"containers" validate:"required,min=1,max=500"`
	Notes      string `form:"notes" validate:"max=2000"`
}

func quotePage(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/quote.html", view.Data{
			"Title": "Request a quote",
			"Form":  quoteForm{Containers: 1},
		})
	}
}

func quoteSubmit(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		var in quoteForm
		problems, err := form.Bind(r, &in)
		if err != nil {
			http.Error(w, "400 bad request", http.StatusBadRequest)
			return
		}

		if problems.Any() {
			a.RenderStatus(w, r, http.StatusUnprocessableEntity, "pages/quote.html", view.Data{
				"Title":    "Request a quote",
				"Problems": problems,
				"Form":     in,
			})
			return
		}

		records, err := a.Store()
		if err != nil {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
			return
		}
		schema, err := a.Describe(QuoteRequest{})
		if err != nil {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
			return
		}

		record := model.Record{
			"company":      in.Company,
			"email":        strings.ToLower(in.Email),
			"origin_code":  strings.ToUpper(in.OriginCode),
			"service":      in.Service,
			"containers":   in.Containers,
			"notes":        in.Notes,
			"submitted_at": time.Now(),
		}
		if _, err := records.Insert(r.Context(), schema, record); err != nil {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
			return
		}

		confirmQuote(a, in)

		view.Flash(r, "success", "Thank you, we will come back with a rate.")
		view.Redirect(w, r, "/quote")
	}
}
GO

	if [ "$with_email" = "1" ]; then
		cat >"$EXAMPLE_DIR/mailer.go" <<'GO'
package main

import (
	"context"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/lib/mail"
)

var quoteSender mail.Sender

func openMailer(a *app.App) error {
	sender, err := a.Settings.Email.Open()
	if err != nil {
		return err
	}
	quoteSender = sender
	return nil
}

func confirmQuote(a *app.App, in quoteForm) {
	if quoteSender == nil {
		return
	}

	message := mail.Message{
		From:    a.Settings.Email.From,
		To:      []string{strings.ToLower(in.Email)},
		ReplyTo: a.Settings.Email.From,
		Subject: "Quote request from " + in.Company,
		Text: fmt.Sprintf("Thank you %s.\n\nWe have your request for %d container(s) by %s from %s.\n",
			in.Company, in.Containers, in.Service, in.OriginCode),
		HTML: fmt.Sprintf("<p>Thank you %s.</p><p>%d container(s) by %s from %s.</p>",
			in.Company, in.Containers, in.Service, in.OriginCode),
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := quoteSender.Send(ctx, message); err != nil {
		log.Printf("quote confirmation to %s failed: %v", in.Email, err)
	}
}
GO
	else
		cat >"$EXAMPLE_DIR/mailer.go" <<'GO'
package main

func confirmQuote(a *appHolder, in quoteForm) {}
GO
		rm -f "$EXAMPLE_DIR/mailer.go"
		cat >"$EXAMPLE_DIR/mailer.go" <<'GO'
package main

import "github.com/farhapartex/coyote/core/app"

func confirmQuote(a *app.App, in quoteForm) {}
GO
	fi

	app_gofmt
}

app_write_public_handlers() {
	cat >"$EXAMPLE_DIR/handlers_public.go" <<'GO'
package main

import (
	"net/http"
	"strings"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/view"
)

type service struct {
	Slug    string
	Title   string
	Summary string
}

var services = []service{
	{Slug: "ocean-freight", Title: "Ocean freight", Summary: "Full and less than container loads on every major lane."},
	{Slug: "air-freight", Title: "Air freight", Summary: "Time critical movements with customs handled end to end."},
	{Slug: "customs-brokerage", Title: "Customs brokerage", Summary: "Declarations, duty deferment and compliance advice."},
}

func serviceBySlug(slug string) (service, bool) {
	for _, entry := range services {
		if entry.Slug == slug {
			return entry, true
		}
	}
	return service{}, false
}

func landingPage(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/landing.html", view.Data{
			"Title":    "Meridian Freight",
			"Services": services,
		})
	}
}

func servicesIndex(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/services.html", view.Data{
			"Title":    "Services",
			"Services": services,
		})
	}
}

func serviceDetail(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		entry, found := serviceBySlug(r.PathValue("slug"))
		if !found {
			renderNotFound(a, w, r)
			return
		}
		a.Render(w, r, "pages/service_detail.html", view.Data{
			"Title":   entry.Title,
			"Service": entry,
		})
	}
}

func trackForm(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, "pages/track.html", view.Data{"Title": "Track a shipment"})
	}
}

func trackResult(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		reference := strings.ToUpper(strings.TrimSpace(r.PathValue("reference")))

		records, err := a.Store()
		if err != nil {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
			return
		}
		schema, err := a.Describe(Shipment{})
		if err != nil {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
			return
		}

		found, err := records.First(r.Context(), schema, model.Query{
			Filters: []model.Filter{{Column: "reference", Op: model.Eq, Value: reference}},
			With:    []string{"origin_id", "destination_id"},
		})
		if err != nil {
			a.Render(w, r, "pages/track_missing.html", view.Data{
				"Title":     "Not found",
				"Reference": reference,
			})
			return
		}

		a.Render(w, r, "pages/track_result.html", view.Data{
			"Title":     reference,
			"Reference": reference,
			"Shipment":  found,
		})
	}
}

func portDetail(a *app.App) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		code := strings.ToUpper(strings.TrimSpace(r.PathValue("code")))

		records, err := a.Store()
		if err != nil {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
			return
		}
		schema, err := a.Describe(Port{})
		if err != nil {
			http.Error(w, "500 internal server error", http.StatusInternalServerError)
			return
		}

		found, err := records.First(r.Context(), schema, model.Query{
			Filters: []model.Filter{{Column: "code", Op: model.Eq, Value: code}},
		})
		if err != nil {
			renderNotFound(a, w, r)
			return
		}

		a.Render(w, r, "pages/port.html", view.Data{
			"Title": found.String("name"),
			"Port":  found,
		})
	}
}

func renderNotFound(a *app.App, w http.ResponseWriter, r *http.Request) {
	a.RenderStatus(w, r, http.StatusNotFound, "pages/notfound.html", view.Data{"Title": "Not found"})
}

func staticPage(a *app.App, template, title string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		a.Render(w, r, template, view.Data{"Title": title})
	}
}
GO
	app_gofmt
}

app_write_public_templates() {
	local stage="${1:-14}"
	local pages="$EXAMPLE_DIR/templates/pages"
	mkdir -p "$pages"

	cat >"$pages/landing.html" <<'HTML'
{{define "content"}}
<h1>Meridian Freight</h1>
<p class="lead">Ocean, air and customs, on one booking reference.</p>
<ul>
  {{range .Services}}<li><a href="/services/{{.Slug}}">{{.Title}}</a> — {{.Summary}}</li>{{end}}
</ul>
<p><a href="{{url "track"}}">Track a shipment</a> · <a href="{{url "quote"}}">Request a quote</a></p>
{{end}}
HTML

	cat >"$pages/services.html" <<'HTML'
{{define "content"}}
<h1>Services</h1>
{{range .Services}}
<article><h2><a href="/services/{{.Slug}}">{{.Title}}</a></h2><p>{{.Summary}}</p></article>
{{end}}
{{end}}
HTML

	cat >"$pages/service_detail.html" <<'HTML'
{{define "content"}}
<h1>{{.Service.Title}}</h1>
<p>{{.Service.Summary}}</p>
<p><a href="{{url "services"}}">All services</a></p>
{{end}}
HTML

	cat >"$pages/track.html" <<'HTML'
{{define "content"}}
<h1>Track a shipment</h1>
<form method="get" action="/track" id="trackform">
  <input type="text" name="reference" placeholder="MRF-000001" required>
  <button type="submit">Track</button>
</form>
{{end}}
HTML

	cat >"$pages/track_result.html" <<'HTML'
{{define "content"}}
<h1>{{.Reference}}</h1>
<dl>
  <dt>Status</dt><dd>{{.Shipment.String "status"}}</dd>
  <dt>Origin</dt><dd>{{.Shipment.String "origin_id__label"}}</dd>
  <dt>Destination</dt><dd>{{.Shipment.String "destination_id__label"}}</dd>
</dl>
{{end}}
HTML

	cat >"$pages/track_missing.html" <<'HTML'
{{define "content"}}
<h1>Not found</h1>
<p>No shipment matches {{.Reference}}.</p>
{{end}}
HTML

	cat >"$pages/port.html" <<'HTML'
{{define "content"}}
<h1>{{.Port.String "name"}}</h1>
<p>Code {{.Port.String "code"}} · {{.Port.String "country"}}</p>
{{end}}
HTML

	cat >"$pages/report.html" <<'HTML'
{{define "content"}}
<h1>Lane report</h1>
<p data-builds="{{.Report.Runs}}">Built by run {{.Report.Runs}}</p>
<table>
  {{range .Report.Rows}}<tr data-status="{{.Status}}"><td>{{.Status}}</td><td>{{.Count}}</td></tr>{{end}}
</table>
{{end}}
HTML

	cat >"$pages/shipments.html" <<'HTML'
{{define "content"}}
<h1>Shipments</h1>
<table>
  <tbody>
  {{range .Shipments}}
  <tr data-reference="{{.String "reference"}}">
    <td>{{.String "reference"}}</td>
    <td>{{.String "status"}}</td>
    <td>{{.String "origin_id__label"}}</td>
  </tr>
  {{end}}
  </tbody>
</table>
{{with .Page}}
{{if and .Enabled (not .Single)}}
<nav class="pager">
  {{if .HasPrev}}<a href="?page={{.Prev}}" rel="prev">Previous</a>{{end}}
  {{range .Numbers}}<a href="?page={{.}}">{{.}}</a>{{end}}
  {{if .HasNext}}<a href="?page={{.Next}}" rel="next">Next</a>{{end}}
</nav>
<p class="counts">Page {{.Number}} of {{.Pages}} · {{.Total}} total · {{.PerPage}} per page</p>
{{end}}
{{end}}
{{end}}
HTML

	cat >"$pages/notfound.html" <<'HTML'
{{define "content"}}
<h1>Page not found</h1>
<p>Nothing lives at this address.</p>
{{end}}
HTML

	if [ "$stage" -ge 19 ]; then
		cat >"$pages/about.html" <<'HTML'
{{define "content"}}
<h1 data-i18n="about">{{.Locale.T "About Meridian"}}</h1>
<p data-i18n="services">{{.Locale.T "Services"}}</p>
<p data-i18n="track">{{.Locale.T "Track a shipment"}}</p>
<p data-i18n="quote">{{.Locale.T "Request a quote"}}</p>
<p data-plural="0">{{.Locale.N "%d shipment on this lane" "%d shipments on this lane" 0}}</p>
<p data-plural="1">{{.Locale.N "%d shipment on this lane" "%d shipments on this lane" 1}}</p>
<p data-plural="2">{{.Locale.N "%d shipment on this lane" "%d shipments on this lane" 2}}</p>
<p data-plural="7">{{.Locale.N "%d shipment on this lane" "%d shipments on this lane" 7}}</p>
<p data-tag="{{.Locale.Tag}}" data-dir="{{.Locale.Direction}}">locale</p>
<nav class="langs">
  {{$path := .Path}}
  {{range .Locale.Available}}<a href="/locale?locale={{.Tag}}&next={{$path}}" data-locale="{{.Tag}}">{{.Name}}</a>{{end}}
</nav>
{{end}}
HTML
	else
		cat >"$pages/about.html" <<'HTML'
{{define "content"}}
<h1>About Meridian</h1>
<p>A freight forwarder built to exercise a framework.</p>
{{end}}
HTML
	fi

	cat >"$pages/contact.html" <<'HTML'
{{define "content"}}
<h1>Contact</h1>
<p>Rotterdam · Singapore · Shanghai</p>
{{end}}
HTML

	if [ "$stage" -ge 15 ]; then
		cat >"$pages/quote.html" <<'HTML'
{{define "content"}}
<h1>Request a quote</h1>
{{with .Problems}}
<ul class="problems">
  {{range $field, $messages := .}}{{range $messages}}<li data-field="{{$field}}">{{$field}}: {{.}}</li>{{end}}{{end}}
</ul>
{{end}}
<form method="post" action="/quote">
  <input type="hidden" name="csrf_token" value="{{.CSRFToken}}">
  <input type="text" name="company" value="{{.Form.Company}}" placeholder="Company">
  <input type="text" name="email" value="{{.Form.Email}}" placeholder="Email">
  <input type="text" name="origin_code" value="{{.Form.OriginCode}}" placeholder="NLRTM">
  <input type="text" name="service" value="{{.Form.Service}}" placeholder="ocean">
  <input type="number" name="containers" value="{{.Form.Containers}}">
  <textarea name="notes">{{.Form.Notes}}</textarea>
  <button type="submit">Request a quote</button>
</form>
{{end}}
HTML
	else
		cat >"$pages/quote.html" <<'HTML'
{{define "content"}}
<h1>Request a quote</h1>
<p>Tell us what you are moving and we will come back with a rate.</p>
{{end}}
HTML
	fi
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
	if [ "$stage" -ge 15 ]; then
		printf 'model.Of(Customer{}), model.Of(Port{}), model.Of(Shipment{}), model.Of(Container{}), model.Of(TrackingEvent{}), model.Of(Invoice{}), model.Of(InvoiceLine{}), model.Of(QuoteRequest{})'
	elif [ "$stage" -ge 12 ]; then
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
		printf '\t"net/http"\n'
		printf '\n'
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
		if [ "$stage" -ge 14 ]; then
			printf '\ta.Get("/{$}", landingPage(a)).Named("home")\n'
			printf '\ta.Get("/services", servicesIndex(a)).Named("services")\n'
			printf '\ta.Get("/services/{slug}", serviceDetail(a)).Named("service.detail")\n'
			printf '\ta.Get("/track", trackForm(a)).Named("track")\n'
			printf '\ta.Get("/track/{reference}", trackResult(a)).Named("track.result")\n'
			printf '\ta.Get("/ports/{code}", portDetail(a)).Named("port.detail")\n'
			if [ "$stage" -ge 17 ]; then
				printf '\ta.Get("/shipments", shipmentList(a)).Named("shipments")\n'
			fi
			if [ "$stage" -ge 18 ]; then
				printf '\ta.Get("/report", laneReportPage(a)).Named("report")\n'
			fi
			if [ "$stage" -ge 19 ]; then
				printf '\ta.Get("/locale", a.Locales().SwitchHandler("/")).Named("locale")\n'
			fi
			if [ "$stage" -ge 20 ]; then
				printf '\n\tif err := openMailer(a); err != nil {\n'
				printf '\t\tlog.Fatal(err)\n'
				printf '\t}\n\n'
			fi
			if [ "$stage" -ge 15 ]; then
				printf '\ta.Get("/quote", quotePage(a), a.CSRF).Named("quote")\n'
				printf '\ta.Post("/quote", quoteSubmit(a), a.CSRF)\n'
			else
				printf '\ta.Get("/quote", staticPage(a, "pages/quote.html", "Request a quote")).Named("quote")\n'
			fi
			printf '\ta.Get("/about", staticPage(a, "pages/about.html", "About")).Named("about")\n'
			printf '\ta.Get("/contact", staticPage(a, "pages/contact.html", "Contact")).Named("contact")\n\n'
			printf '\ta.SetNotFound(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {\n'
			printf '\t\trenderNotFound(a, w, r)\n'
			printf '\t}))\n\n'
		else
			printf '\ta.Get("/{$}", func(w http.ResponseWriter, r *http.Request) {\n'
			printf '\t\ta.Render(w, r, "pages/home.html", app.Data{"Title": "Home"})\n'
			printf '\t}).Named("home")\n\n'
		fi
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
