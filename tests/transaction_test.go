package tests

import (
	"bytes"
	"errors"
	"testing"

	"github.com/farhapartex/coyote/core/app"
	"github.com/farhapartex/coyote/core/model"
	"github.com/farhapartex/coyote/core/settings"
)

var errRolledBack = errors.New("rolled back on purpose")

func txApp(t *testing.T) (*app.App, *model.Schema) {
	t.Helper()
	a := app.NewFrom(devSettings(t, func(s *settings.Settings) {
		s.Uploads.Enabled = true
		s.Uploads.Allowed = []string{"image/png"}
	}))
	a.RegisterModel(model.Of(Widget2{}))
	syncSchema(t, a)

	schema, err := a.Describe(Widget2{})
	if err != nil {
		t.Fatal(err)
	}
	return a, schema
}

func TestTransactionCommitsEverythingOrNothing(t *testing.T) {
	a, schema := txApp(t)

	err := a.Transaction(t.Context(), func(tx *app.Tx) error {
		if _, err := tx.Store().Insert(t.Context(), schema, model.Record{"id": "one", "name": "One"}); err != nil {
			return err
		}
		_, err := tx.Store().Insert(t.Context(), schema, model.Record{"id": "two", "name": "Two"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}

	records, _ := a.Store()
	total, err := records.Count(t.Context(), schema, model.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 2 {
		t.Errorf("committed %d rows, want 2", total)
	}
}

func TestTransactionRollsBackOnError(t *testing.T) {
	a, schema := txApp(t)

	err := a.Transaction(t.Context(), func(tx *app.Tx) error {
		if _, err := tx.Store().Insert(t.Context(), schema, model.Record{"id": "one", "name": "One"}); err != nil {
			return err
		}
		return errRolledBack
	})
	if !errors.Is(err, errRolledBack) {
		t.Fatalf("error = %v, want the returned one", err)
	}

	records, _ := a.Store()
	total, err := records.Count(t.Context(), schema, model.Query{})
	if err != nil {
		t.Fatal(err)
	}
	if total != 0 {
		t.Errorf("%d rows survived a rollback", total)
	}
}

func TestTransactionPromotesKeptFilesOnlyOnCommit(t *testing.T) {
	a, schema := txApp(t)
	body := pngBytes(t, 3, 3)

	file, err := a.Uploads.Store(t.Context(), bytes.NewReader(body), "a.png", "image/png", int64(len(body)))
	if err != nil {
		t.Fatal(err)
	}
	ref := file.Ref

	err = a.Transaction(t.Context(), func(tx *app.Tx) error {
		tx.Keep(&ref)
		_, err := tx.Store().Insert(t.Context(), schema, model.Record{"id": "one", "name": "One"})
		return err
	})
	if err != nil {
		t.Fatal(err)
	}
	if !ref.Committed() {
		t.Errorf("a kept file should be promoted after commit, got %q", ref)
	}
	if !a.Uploads.Storage().Exists(t.Context(), string(ref)) {
		t.Error("the promoted file should be on disk")
	}

	second, err := a.Uploads.Store(t.Context(), bytes.NewReader(pngBytes(t, 4, 4)), "b.png", "image/png", 0)
	if err != nil {
		t.Fatal(err)
	}
	staged := second.Ref

	err = a.Transaction(t.Context(), func(tx *app.Tx) error {
		tx.Keep(&staged)
		return errRolledBack
	})
	if !errors.Is(err, errRolledBack) {
		t.Fatalf("error = %v", err)
	}
	if staged.Committed() {
		t.Error("a rolled-back transaction must not promote its files")
	}
	if !a.Uploads.Storage().Exists(t.Context(), string(staged)) {
		t.Error("the file should still be in staging for the sweeper to reclaim")
	}
	if !staged.Staged() {
		t.Errorf("ref = %q, want it left in staging", staged)
	}
}

func TestTransactionRollsBackOnPanic(t *testing.T) {
	a, schema := txApp(t)

	func() {
		defer func() {
			if recover() == nil {
				t.Error("the panic should reach the caller")
			}
		}()
		_ = a.Transaction(t.Context(), func(tx *app.Tx) error {
			tx.Store().Insert(t.Context(), schema, model.Record{"id": "one", "name": "One"})
			panic("boom")
		})
	}()

	records, _ := a.Store()
	total, _ := records.Count(t.Context(), schema, model.Query{})
	if total != 0 {
		t.Errorf("%d rows survived a panic", total)
	}
}

func TestStoreWithTxIsScopedToTheTransaction(t *testing.T) {
	a, schema := txApp(t)
	records, _ := a.Store()

	var inside model.Store
	err := a.Transaction(t.Context(), func(tx *app.Tx) error {
		inside = tx.Store()
		_, err := inside.Insert(t.Context(), schema, model.Record{"id": "one", "name": "One"})
		if err != nil {
			return err
		}
		total, err := inside.Count(t.Context(), schema, model.Query{})
		if err != nil {
			return err
		}
		if total != 1 {
			t.Errorf("the transaction should see its own write, counted %d", total)
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
	if inside == records {
		t.Error("WithTx should hand back a scoped store, not the shared one")
	}
}
