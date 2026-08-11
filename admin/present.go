package admin

import (
	"fmt"
	"strconv"
	"time"

	"github.com/farhapartex/coyote/core/settings"
)

type settingRow struct {
	Name  string
	Value string
}

type settingGroup struct {
	Name string
	Rows []settingRow
}

func boolText(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func poolText(n int) string {
	if n == 0 {
		return "driver default"
	}
	return strconv.Itoa(n)
}

func durationText(d time.Duration) string {
	if d == 0 {
		return "none"
	}
	return d.String()
}

func orDash(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func storeName(v any) string {
	if v == nil {
		return "—"
	}
	return fmt.Sprintf("%T", v)
}

func templateSource(s settings.Settings) string {
	switch {
	case s.Templates.Dir != "":
		return "Dir " + s.Templates.Dir
	case s.Templates.FS != nil:
		return fmt.Sprintf("FS %T", s.Templates.FS)
	default:
		return "not configured"
	}
}

func staticSource(s settings.Settings) string {
	switch {
	case s.Static.Dir != "":
		return "Dir " + s.Static.Dir
	case s.Static.FS != nil:
		return fmt.Sprintf("FS %T", s.Static.FS)
	default:
		return "not configured"
	}
}
