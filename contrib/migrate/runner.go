package migrate

import (
	"fmt"

	"github.com/farhapartex/coyote/core/model"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Runner struct {
	handle *gorm.DB
	models []model.Model
}

func New(handle *gorm.DB, models []model.Model) *Runner {
	return &Runner{handle: quiet(handle), models: models}
}

func quiet(handle *gorm.DB) *gorm.DB {
	if handle == nil {
		return nil
	}
	return handle.Session(&gorm.Session{Logger: logger.Discard})
}

func (r *Runner) Models() []model.Model { return r.models }

func (r *Runner) Plan() (Report, error) {
	inspect := inspector{handle: r.handle}
	var report Report
	for _, m := range r.models {
		change, err := inspect.change(m)
		if err != nil {
			return Report{}, err
		}
		if !change.Empty() {
			report.Changes = append(report.Changes, change)
		}
	}
	return report, nil
}

func (r *Runner) ApplyChange(change TableChange) error {
	target, err := r.modelFor(inspector{handle: r.handle}, change.Table)
	if err != nil {
		return err
	}
	if err := r.handle.AutoMigrate(target.Entity); err != nil {
		return fmt.Errorf("coyote/migrate: %s: %w", change.Table, err)
	}
	return nil
}

func (r *Runner) Apply() (Report, error) {
	report, err := r.Plan()
	if err != nil {
		return Report{}, err
	}
	for _, change := range report.Changes {
		if err := r.ApplyChange(change); err != nil {
			return report, err
		}
	}
	return report, nil
}

func (r *Runner) modelFor(inspect inspector, table string) (model.Model, error) {
	for _, m := range r.models {
		name, err := inspect.tableName(m)
		if err != nil {
			return model.Model{}, err
		}
		if name == table {
			return m, nil
		}
	}
	return model.Model{}, fmt.Errorf("coyote/migrate: no model for table %s", table)
}
