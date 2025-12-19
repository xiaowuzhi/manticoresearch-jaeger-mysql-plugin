package tlog

import (
	"github.com/rs/zerolog"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/trace"
)

// Log 全局日志实例，使用方式: tlog.Log.Info(ctx, "msg")
var Log = &Logger{}

type Logger struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	console  bool
	zlog     zerolog.Logger
}

const (
	eventError = "error"
	eventInfo  = "info"
	eventDebug = "debug"
	eventWarn  = "warn"
)


