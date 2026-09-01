package db

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"gorm.io/gorm/logger"
)

type slogLogger struct {
	log       *slog.Logger
	level     logger.LogLevel
	statement bool
}

func newLogger(log *slog.Logger, level logger.LogLevel, statement bool) logger.Interface {
	if log == nil {
		log = slog.Default()
	}
	return &slogLogger{log: log, level: level, statement: statement}
}

func (l *slogLogger) LogMode(level logger.LogLevel) logger.Interface {
	clone := *l
	clone.level = level
	return &clone
}

func (l *slogLogger) Info(_ context.Context, msg string, data ...any) {
	if l.level >= logger.Info {
		l.log.Info("gorm: "+msg, slog.Any("data", data))
	}
}

func (l *slogLogger) Warn(_ context.Context, msg string, data ...any) {
	if l.level >= logger.Warn {
		l.log.Warn("gorm: "+msg, slog.Any("data", data))
	}
}

func (l *slogLogger) Error(_ context.Context, msg string, data ...any) {
	if l.level >= logger.Error {
		l.log.Error("gorm: "+msg, slog.Any("data", data))
	}
}

func (l *slogLogger) Trace(_ context.Context, begin time.Time, fc func() (string, int64), err error) {
	if l.level <= logger.Silent {
		return
	}
	statement, rows := fc()
	attrs := []any{
		slog.Int64("rows", rows),
		slog.Duration("took", time.Since(begin).Round(time.Microsecond)),
	}
	if l.statement {
		attrs = append(attrs, slog.String("sql", statement))
	}
	switch {
	case err != nil && !errors.Is(err, logger.ErrRecordNotFound):
		l.log.Error("gorm query failed", append(attrs, slog.Any("error", err))...)
	case l.level >= logger.Info:
		l.log.Debug("gorm query", attrs...)
	}
}
