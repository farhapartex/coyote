package db

import (
	"log/slog"

	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

type Options struct {
	Logger *slog.Logger
	Debug  bool
}

func gormConfig(opts Options) *gorm.Config {
	level := logger.Warn
	if opts.Debug {
		level = logger.Info
	}
	return &gorm.Config{
		Logger:                                   newLogger(opts.Logger, level),
		SkipDefaultTransaction:                   true,
		DisableForeignKeyConstraintWhenMigrating: false,
	}
}
