package app

import (
	"net/http"
	"path/filepath"
	"strings"

	"github.com/farhapartex/coyote/core/storage"
	"github.com/farhapartex/coyote/core/upload"
)

func uploadService(s Settings) *upload.Service {
	if !s.Uploads.Enabled {
		return nil
	}

	backend := s.Uploads.Storage
	if backend == nil {
		dir := s.Uploads.Dir
		if !filepath.IsAbs(dir) {
			dir = s.Path(dir)
		}
		backend = storage.NewFileSystem(dir)
	}

	return upload.New(upload.Options{
		Storage: backend,
		Rules: upload.Rules{
			MaxSize:   s.Uploads.MaxSize,
			Allowed:   s.Uploads.Allowed,
			MaxPixels: s.Uploads.MaxPixels,
		},
		Path:     s.Uploads.Path,
		StageTTL: s.Uploads.StageTTL,
		TrashTTL: s.Uploads.TrashTTL,
	})
}

func (a *App) mountUploads() {
	if a.Uploads == nil || !a.Settings.Uploads.Serve {
		return
	}
	prefix := a.Settings.Uploads.URL
	handler := a.Uploads.Handler(upload.HandlerOptions{
		Prefix:  prefix,
		Private: a.Settings.Uploads.Private,
		Secret:  a.Settings.SecretKey,
	})
	a.Router.Handle(http.MethodGet, strings.TrimSuffix(prefix, "/")+"/", handler)
}

func (a *App) MediaURL(ref upload.Ref) string {
	if a.Uploads == nil {
		return ""
	}
	if a.Settings.Uploads.Private {
		return a.Uploads.SignedURL(a.Settings.Uploads.URL, a.Settings.SecretKey, ref, a.Settings.Uploads.StageTTL)
	}
	return a.Uploads.URL(a.Settings.Uploads.URL, ref)
}
