package i18n

import "time"

type Formatter interface {
	Date(t time.Time) string
	Time(t time.Time) string
	DateTime(t time.Time) string
	Number(value any) string
	Money(value any, currency string) string
}
