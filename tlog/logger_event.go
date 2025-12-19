package tlog

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Event 添加自定义事件
func (l *Logger) Event(ctx context.Context, name string, kvs ...interface{}) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return
	}
	attrs := make([]attribute.KeyValue, 0)
	for i := 0; i+1 < len(kvs); i += 2 {
		key := fmt.Sprintf("%v", kvs[i])
		val := fmt.Sprintf("%v", kvs[i+1])
		attrs = append(attrs, attribute.String(key, val))
	}
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SQL 记录 SQL 查询
func (l *Logger) SQL(ctx context.Context, sql string, duration string) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return
	}
	span.AddEvent("sql", trace.WithAttributes(
		attribute.String("statement", sql),
		attribute.String("duration", duration),
	))
}


