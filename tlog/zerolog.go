package tlog

import (
	"context"
	"os"
	"time"

	"github.com/rs/zerolog"
)

func initZerolog(cfg Config) {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	// 如果外部传入自定义 logger（已包含上报 hook / 输出），直接用
	if cfg.ZLogger != nil {
		Log.zlog = *cfg.ZLogger
		return
	}

	w := cfg.ZWriter
	if w == nil {
		// 默认输出到 stdout；调用方可用 io.MultiWriter(...) 实现“上报 + 控制台”
		w = os.Stdout
	}

	level := cfg.ZLevel
	if level == zerolog.NoLevel {
		level = zerolog.InfoLevel
	}

	Log.zlog = zerolog.New(w).
		Level(level).
		With().
		Timestamp().
		Str("service", cfg.ServiceName).
		Str("host", cfg.HostName).
		Str("otel_endpoint", cfg.Endpoint).
		Logger()
}

func (l *Logger) z(ctx context.Context) zerolog.Logger {
	// 如果 ctx 上有 logger（zerolog 的最佳实践），优先使用
	if ctx != nil {
		if zl := zerolog.Ctx(ctx); zl != nil {
			return *zl
		}
	}

	// 否则 fallback 到全局 logger，并尽量补上 trace 字段
	base := l.zlog
	if ctx != nil {
		tid, sid := traceIDs(ctx)
		if tid != "" {
			base = base.With().Str("trace_id", tid).Str("span_id", sid).Logger()
		}
	}
	return base
}

func (l *Logger) withTrace(ctx context.Context) context.Context {
	tid, sid := traceIDs(ctx)
	if tid == "" {
		return ctx
	}
	zl := l.zlog.With().Str("trace_id", tid).Str("span_id", sid).Logger()
	return zl.WithContext(ctx)
}


