package view

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
)

const DefaultMaxJSONBody = 1 << 20

var ErrBodyTooLarge = errors.New("coyote/view: request body is too large")

func JSON(w http.ResponseWriter, status int, payload any) error {
	body, err := json.Marshal(payload)
	if err != nil {
		http.Error(w, "500 internal server error", http.StatusInternalServerError)
		return err
	}
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("X-Content-Type-Options", "nosniff")
	w.WriteHeader(status)
	_, err = w.Write(body)
	return err
}

func JSONError(w http.ResponseWriter, status int, message string) error {
	if message == "" {
		message = http.StatusText(status)
	}
	return JSON(w, status, map[string]string{"error": message})
}

func JSONProblems(w http.ResponseWriter, status int, problems map[string][]string) error {
	return JSON(w, status, map[string]any{
		"error":    http.StatusText(status),
		"problems": problems,
	})
}

func Decode(r *http.Request, target any) error {
	return DecodeLimit(r, target, DefaultMaxJSONBody)
}

func DecodeLimit(r *http.Request, target any, maxBytes int64) error {
	if r.Body == nil {
		return errors.New("coyote/view: request has no body")
	}
	if kind := r.Header.Get("Content-Type"); kind != "" && !strings.HasPrefix(kind, "application/json") {
		return fmt.Errorf("coyote/view: expected application/json, got %q", kind)
	}

	body := io.Reader(r.Body)
	if maxBytes > 0 {
		body = http.MaxBytesReader(nil, r.Body, maxBytes)
	}

	decoder := json.NewDecoder(body)
	decoder.DisallowUnknownFields()

	if err := decoder.Decode(target); err != nil {
		var tooLarge *http.MaxBytesError
		if errors.As(err, &tooLarge) {
			return fmt.Errorf("%w: limit is %d bytes", ErrBodyTooLarge, maxBytes)
		}
		return fmt.Errorf("coyote/view: %w", err)
	}
	if err := decoder.Decode(&struct{}{}); !errors.Is(err, io.EOF) {
		return errors.New("coyote/view: body must hold a single JSON value")
	}
	return nil
}

func WantsJSON(r *http.Request) bool {
	if strings.HasPrefix(r.Header.Get("Content-Type"), "application/json") {
		return true
	}
	accept := r.Header.Get("Accept")
	return strings.Contains(accept, "application/json") && !strings.Contains(accept, "text/html")
}
