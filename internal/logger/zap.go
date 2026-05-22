package logger

import (
	"context"
	"strings"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"

	"github.com/abduromanov2020/tasks-api/internal/domain"
)

type ctxKey struct{}

var redactedKeys = map[string]struct{}{
	"authorization": {}, "password": {}, "password_hash": {},
	"token": {}, "jwt": {}, "secret": {},
}

func New(level, env string) (*zap.Logger, error) {
	lvl, err := zapcore.ParseLevel(strings.ToLower(level))
	if err != nil {
		lvl = zapcore.InfoLevel
	}
	cfg := zap.NewProductionConfig()
	if env == "development" {
		cfg.Encoding = "console"
		cfg.EncoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	}
	cfg.Level = zap.NewAtomicLevelAt(lvl)
	cfg.EncoderConfig.TimeKey = "ts"
	cfg.EncoderConfig.MessageKey = "msg"
	cfg.EncoderConfig.LevelKey = "level"
	cfg.EncoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	cfg.DisableStacktrace = true
	return cfg.Build()
}

// Wrap returns a domain.Logger backed by zap with redaction of sensitive keys.
func Wrap(z *zap.Logger) domain.Logger { return zapLogger{l: z} }

// Into stores a domain.Logger in the context.
func Into(ctx context.Context, l domain.Logger) context.Context {
	return context.WithValue(ctx, ctxKey{}, l)
}

// FromCtx fetches the logger stored in ctx, or returns a no-op-safe nop logger.
func FromCtx(ctx context.Context) domain.Logger {
	if v, ok := ctx.Value(ctxKey{}).(domain.Logger); ok && v != nil {
		return v
	}
	return Wrap(zap.NewNop())
}

type zapLogger struct{ l *zap.Logger }

func (z zapLogger) Debug(msg string, kv ...any) { z.l.Debug(msg, fields(kv)...) }
func (z zapLogger) Info(msg string, kv ...any)  { z.l.Info(msg, fields(kv)...) }
func (z zapLogger) Warn(msg string, kv ...any)  { z.l.Warn(msg, fields(kv)...) }
func (z zapLogger) Error(msg string, kv ...any) { z.l.Error(msg, fields(kv)...) }

func (z zapLogger) With(kv ...any) domain.Logger {
	return zapLogger{l: z.l.With(fields(kv)...)}
}

func fields(kv []any) []zap.Field {
	if len(kv) == 0 {
		return nil
	}
	out := make([]zap.Field, 0, len(kv)/2)
	for i := 0; i+1 < len(kv); i += 2 {
		k, ok := kv[i].(string)
		if !ok {
			continue
		}
		if _, redact := redactedKeys[strings.ToLower(k)]; redact {
			out = append(out, zap.String(k, "[REDACTED]"))
			continue
		}
		out = append(out, zap.Any(k, kv[i+1]))
	}
	return out
}
