package admin

import (
	"errors"
	"net/http"

	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/upload"
)

const uploadMemory = 32 << 20

func (a *Admin) hasFiles(schema *model.Schema) bool {
	for _, field := range schema.FormFields() {
		if field.Kind == model.KindFile {
			return true
		}
	}
	return false
}

func (a *Admin) attachFiles(r *http.Request, entry managed, record model.Record, existing model.Record) map[string]string {
	problems := map[string]string{}
	if a.app.Uploads == nil {
		return problems
	}

	for _, field := range entry.schema.FormFields() {
		if field.Kind != model.KindFile {
			continue
		}

		file, err := a.app.Uploads.AcceptTo(r, field.Column, field.UploadPath)
		if errors.Is(err, upload.ErrNoFile) {
			if existing != nil {
				if current := existing.String(field.Column); current != "" {
					record[field.Column] = current
				}
			}
			if r.PostForm.Get(field.Column+"__clear") != "" {
				record[field.Column] = ""
			}
			continue
		}
		if err != nil {
			problems[field.Column] = uploadProblem(err)
			continue
		}

		ref := file.Ref
		if err := a.app.Uploads.Commit(r.Context(), &ref); err != nil {
			problems[field.Column] = "could not be stored"
			continue
		}
		record[field.Column] = string(ref)
	}
	return problems
}

func uploadProblem(err error) string {
	switch {
	case errors.Is(err, upload.ErrTooLarge):
		return "is larger than the limit"
	case errors.Is(err, upload.ErrTypeNotAllow):
		return "is not an accepted file type"
	case errors.Is(err, upload.ErrTypeMismatch):
		return "does not match the type it claims to be"
	case errors.Is(err, upload.ErrTooManyPixel):
		return "has too many pixels"
	case errors.Is(err, upload.ErrEmptyFile):
		return "is empty"
	}
	return "could not be uploaded"
}
