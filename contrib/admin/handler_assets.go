package admin

import (
	"bytes"
	"net/http"
	"time"
)

const assetCacheControl = "public, max-age=86400"

func (a *Admin) favicon(w http.ResponseWriter, r *http.Request) {
	raw, err := templateFS.ReadFile("assets/favicon.png")
	if err != nil {
		http.NotFound(w, r)
		return
	}
	w.Header().Set("Content-Type", "image/png")
	w.Header().Set("Cache-Control", assetCacheControl)
	http.ServeContent(w, r, "favicon.png", a.app.Started.Truncate(time.Second), bytes.NewReader(raw))
}
