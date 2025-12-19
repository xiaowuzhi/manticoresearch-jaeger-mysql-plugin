package tlog

import (
	"context"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"
)

// Tag 设置单个标签
func (l *Logger) Tag(ctx context.Context, key string, value interface{}) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return
	}
	span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", value)))
}

// Tags 批量设置标签
func (l *Logger) Tags(ctx context.Context, kvs ...interface{}) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return
	}
	for i := 0; i+1 < len(kvs); i += 2 {
		key := fmt.Sprintf("%v", kvs[i])
		val := fmt.Sprintf("%v", kvs[i+1])
		span.SetAttributes(attribute.String(key, val))
	}
}

// TagsMap 使用 map 批量设置标签
func (l *Logger) TagsMap(ctx context.Context, tags map[string]interface{}) {
	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return
	}
	for k, v := range tags {
		span.SetAttributes(attribute.String(k, fmt.Sprintf("%v", v)))
	}
}

// HTTP 设置 HTTP 标签
func (l *Logger) HTTP(ctx context.Context, method, path string, statusCode int) {
	l.Tags(ctx, "http.method", method, "http.url", path, "http.status_code", statusCode)
}

// User 设置用户标签
func (l *Logger) User(ctx context.Context, userID, userName string) {
	l.Tags(ctx, "user.id", userID, "user.name", userName)
}


