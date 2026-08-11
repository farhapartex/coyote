package admin

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/farhapartex/coyote/core/view"
)

func (a *Admin) settingsView(w http.ResponseWriter, r *http.Request) {
	s := a.app.Settings
	secretKey := "not set"
	if s.SecretKey != "" {
		secretKey = "set, " + strconv.Itoa(len(s.SecretKey)) + " characters hidden"
		if s.SecretKeyGenerated() {
			secretKey += " (generated for this run)"
		}
	}
	allowedHosts := "any host (Debug, no AllowedHosts set)"
	if len(s.AllowedHosts) > 0 {
		allowedHosts = strings.Join(s.AllowedHosts, ", ")
	}
	groups := []settingGroup{
		{"Core", []settingRow{
			{"Debug", boolText(s.Debug)},
			{"SecretKey", secretKey},
			{"AllowedHosts", allowedHosts},
			{"BaseDir", s.BaseDir},
		}},
	}
	for i, db := range s.Databases {
		redacted := db.Redacted()
		name := "Databases[" + strconv.Itoa(i) + "] " + redacted.Alias
		if i == 0 {
			name += " (default)"
		}
		rows := []settingRow{
			{"Engine", string(redacted.Engine)},
			{"Name", redacted.Name},
		}
		if !redacted.IsSQLite() {
			rows = append(rows,
				settingRow{"Host", redacted.Host},
				settingRow{"Port", strconv.Itoa(redacted.Port)},
				settingRow{"User", orDash(redacted.User)},
				settingRow{"Password", orDash(redacted.Password)},
			)
		}
		rows = append(rows,
			settingRow{"MaxOpenConns", poolText(redacted.MaxOpenConns)},
			settingRow{"MaxIdleConns", poolText(redacted.MaxIdleConns)},
			settingRow{"ConnMaxLifetime", durationText(redacted.ConnMaxLifetime)},
			settingRow{"ConnMaxIdleTime", durationText(redacted.ConnMaxIdleTime)},
			settingRow{"DSN", redacted.DSN()},
		)
		groups = append(groups, settingGroup{name, rows})
	}

	a.render(w, r, http.StatusOK, "settings.html", view.Data{
		"Nav": "settings",
		"Groups": append(groups, []settingGroup{
			{"Server", []settingRow{
				{"Addr", s.Addr()},
				{"ReadTimeout", durationText(s.Server.ReadTimeout)},
				{"WriteTimeout", durationText(s.Server.WriteTimeout)},
				{"IdleTimeout", durationText(s.Server.IdleTimeout)},
				{"ReadHeaderTimeout", durationText(s.Server.ReadHeaderTimeout)},
				{"ShutdownTimeout", durationText(s.Server.ShutdownTimeout)},
			}},
			{"Sessions", []settingRow{
				{"CookieName", s.Sessions.CookieName},
				{"Lifetime", s.Sessions.Lifetime.String()},
				{"Rolling", boolText(s.Sessions.Rolling)},
				{"Secure", boolText(s.Sessions.Secure)},
				{"HTTPOnly", boolText(s.Sessions.HTTPOnly)},
				{"SameSite", string(s.Sessions.SameSite)},
				{"Path", s.Sessions.Path},
				{"Domain", orDash(s.Sessions.Domain)},
				{"CleanupInterval", durationText(s.Sessions.CleanupInterval)},
				{"Store", storeName(a.app.SessionStore())},
			}},
			{"Auth", []settingRow{
				{"LoginURL", orDash(s.Auth.LoginURL)},
				{"PasswordMinLength", strconv.Itoa(s.Auth.PasswordMinLength)},
				{"PBKDF2Iterations", strconv.Itoa(s.Auth.PBKDF2Iterations)},
				{"UserStore", storeName(a.app.Auth.Users())},
			}},
			{"Templates", []settingRow{
				{"Layout", s.Templates.Layout},
				{"Shared", strings.Join(s.Templates.Shared, ", ")},
				{"Source", templateSource(s)},
				{"AutoReload", boolText(s.AutoReloadTemplates())},
			}},
			{"Static", []settingRow{
				{"URL", s.Static.URL},
				{"Source", staticSource(s)},
			}},
			{"Admin", []settingRow{
				{"Prefix", s.Admin.Prefix},
				{"SiteName", s.Admin.SiteName},
				{"Tagline", orDash(s.Admin.Tagline)},
			}},
			{"Logging", []settingRow{
				{"Level", s.Logging.Level},
				{"Format", s.Logging.Format},
			}},
		}...),
	})
}
