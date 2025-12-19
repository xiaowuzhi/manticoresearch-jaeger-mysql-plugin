package tlog

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// Start 开始一个 Span
func (l *Logger) Start(ctx context.Context, name string) (context.Context, trace.Span) {
	if l.tracer == nil {
		return ctx, nil
	}
	ctx, span := l.tracer.Start(ctx, name)
	// 按 zerolog 最佳实践：把 logger 放进 ctx，后续 Info/Error 直接从 ctx 取（并带 trace_id/span_id）
	ctx = l.withTrace(ctx)
	return ctx, span
}

// End 结束 Span
func (l *Logger) End(span trace.Span) {
	if span != nil {
		span.End()
	}
}


