package tlog

import (
	"context"

	"go.opentelemetry.io/otel/trace"
)

// TraceIDs 暴露 trace_id / span_id（如果 ctx 中没有有效 Span，则返回空字符串）
func TraceIDs(ctx context.Context) (traceID string, spanID string) {
	return traceIDs(ctx)
}

// TraceID 暴露 trace_id
func TraceID(ctx context.Context) string {
	tid, _ := traceIDs(ctx)
	return tid
}

// SpanID 暴露 span_id
func SpanID(ctx context.Context) string {
	_, sid := traceIDs(ctx)
	return sid
}

func traceIDs(ctx context.Context) (traceID string, spanID string) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return "", ""
	}
	return sc.TraceID().String(), sc.SpanID().String()
}


