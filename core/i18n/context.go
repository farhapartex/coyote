package i18n

import (
	"context"
	"net/http"
)

type contextKey struct{}

var localeContextKey contextKey

func WithLocale(ctx context.Context, locale *Locale) context.Context {
	return context.WithValue(ctx, localeContextKey, locale)
}

func From(ctx context.Context) *Locale {
	locale, _ := ctx.Value(localeContextKey).(*Locale)
	return locale
}

func FromRequest(r *http.Request) *Locale {
	if r == nil {
		return nil
	}
	return From(r.Context())
}

func T(ctx context.Context, msgid string) string {
	return From(ctx).T(msgid)
}

func Tf(ctx context.Context, msgid string, args ...any) string {
	return From(ctx).Tf(msgid, args...)
}

func TC(ctx context.Context, context_, msgid string) string {
	return From(ctx).TC(context_, msgid)
}

func N(ctx context.Context, singular, plural string, count int, args ...any) string {
	return From(ctx).N(singular, plural, count, args...)
}

func Tag(ctx context.Context) string {
	return From(ctx).Tag()
}
