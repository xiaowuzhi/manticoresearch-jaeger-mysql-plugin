package tlog

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// ============================================================
// Span 操作
// ============================================================

// Start 开始一个 Span
func (l *Logger) Start(ctx context.Context, name string) (context.Context, trace.Span) {
	if l.tracer == nil {
		return ctx, nil
	}
	ctx, span := l.tracer.Start(ctx, name)
	// 把带 trace_id 的 logger 放入 ctx
	ctx = l.ctxWithTrace(ctx)
	return ctx, span
}

// End 结束 Span
func (l *Logger) End(span trace.Span) {
	if span != nil {
		span.End()
	}
}

// ============================================================
// 日志方法
// ============================================================

// Info 记录信息
func (l *Logger) Info(ctx context.Context, msg any, kvs ...any) {
	l.log(ctx, "info", msg, kvs...)
}

// Warn 记录警告
func (l *Logger) Warn(ctx context.Context, msg any, kvs ...any) {
	l.log(ctx, "warn", msg, kvs...)
}

// Debug 记录调试
func (l *Logger) Debug(ctx context.Context, msg any, kvs ...any) {
	l.log(ctx, "debug", msg, kvs...)
}

// Error 记录错误
func (l *Logger) Error(ctx context.Context, err any, kvs ...any) {
	// zerolog 输出
	zl := l.z(ctx)
	ev := zl.Error()
	switch v := err.(type) {
	case error:
		ev = ev.Err(v)
	case nil:
		ev = ev.Str("err", "<nil>")
	default:
		ev = ev.Interface("err", v)
	}
	l.zFields(ev, kvs...).Msg(fmt.Sprint(err))

	// span 记录
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return
	}

	e := toError(err)
	span.RecordError(e)
	span.SetStatus(codes.Error, e.Error())
	span.AddEvent("error", trace.WithAttributes(
		attribute.String("error", e.Error()),
	))
}

// Errorf 格式化错误
func (l *Logger) Errorf(ctx context.Context, format string, args ...any) {
	l.Error(ctx, fmt.Errorf(format, args...))
}

// ============================================================
// 标签方法
// ============================================================

// Tag 设置单个标签
func (l *Logger) Tag(ctx context.Context, key string, value any) {
	if span := trace.SpanFromContext(ctx); span.SpanContext().IsValid() {
		span.SetAttributes(toAttr(key, value))
	}
}

// Tags 批量设置标签 (key, value, key, value, ...)
func (l *Logger) Tags(ctx context.Context, kvs ...any) {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return
	}
	for i := 0; i+1 < len(kvs); i += 2 {
		span.SetAttributes(toAttr(fmt.Sprint(kvs[i]), kvs[i+1]))
	}
}

// HTTP 设置 HTTP 标签
func (l *Logger) HTTP(ctx context.Context, method, path string, status int) {
	l.Tags(ctx, "http.method", method, "http.path", path, "http.status", status)
}

// User 设置用户标签
func (l *Logger) User(ctx context.Context, id, name string) {
	l.Tags(ctx, "user.id", id, "user.name", name)
}

// ============================================================
// 事件方法
// ============================================================

// Event 添加自定义事件
func (l *Logger) Event(ctx context.Context, name string, kvs ...any) {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return
	}
	attrs := l.buildAttrs(kvs...)
	span.AddEvent(name, trace.WithAttributes(attrs...))
}

// SQL 记录 SQL 查询
func (l *Logger) SQL(ctx context.Context, sql, duration string) {
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return
	}
	span.AddEvent("sql", trace.WithAttributes(
		attribute.String("statement", sql),
		attribute.String("duration", duration),
	))
}

// ============================================================
// 内部方法
// ============================================================

func (l *Logger) log(ctx context.Context, level string, msg any, kvs ...any) {
	// zerolog 输出
	zl := l.z(ctx)
	var ev *zerolog.Event
	switch level {
	case "info":
		ev = zl.Info()
	case "warn":
		ev = zl.Warn()
	case "debug":
		ev = zl.Debug()
	default:
		ev = zl.Info()
	}
	l.zFields(ev, kvs...).Msg(fmt.Sprint(msg))

	// span 事件
	span := trace.SpanFromContext(ctx)
	if !span.SpanContext().IsValid() {
		return
	}
	attrs := append([]attribute.KeyValue{attribute.String("msg", fmt.Sprint(msg))}, l.buildAttrs(kvs...)...)
	span.AddEvent(level, trace.WithAttributes(attrs...))
}

func (l *Logger) z(ctx context.Context) zerolog.Logger {
	if ctx != nil {
		if zl := zerolog.Ctx(ctx); zl != nil {
			return *zl
		}
	}
	// fallback: 补上 trace 信息
	base := l.zlog
	if ctx != nil {
		if tid := TraceID(ctx); tid != "" {
			base = base.With().Str("trace_id", tid).Str("span_id", SpanID(ctx)).Logger()
		}
	}
	return base
}

func (l *Logger) ctxWithTrace(ctx context.Context) context.Context {
	tid := TraceID(ctx)
	if tid == "" {
		return ctx
	}
	zl := l.zlog.With().Str("trace_id", tid).Str("span_id", SpanID(ctx)).Logger()
	return zl.WithContext(ctx)
}

func (l *Logger) zFields(ev *zerolog.Event, kvs ...any) *zerolog.Event {
	if ev == nil || len(kvs) == 0 {
		return ev
	}
	// 支持 map[string]any
	if len(kvs) == 1 {
		if m, ok := kvs[0].(map[string]any); ok {
			for k, v := range m {
				ev = ev.Interface(k, v)
			}
			return ev
		}
	}
	// key-value 形式
	for i := 0; i+1 < len(kvs); i += 2 {
		ev = ev.Interface(fmt.Sprint(kvs[i]), kvs[i+1])
	}
	return ev
}

func (l *Logger) buildAttrs(kvs ...any) []attribute.KeyValue {
	var attrs []attribute.KeyValue
	// 支持 map[string]any
	if len(kvs) == 1 {
		if m, ok := kvs[0].(map[string]any); ok {
			for k, v := range m {
				attrs = append(attrs, toAttr(k, v))
			}
			return attrs
		}
	}
	// key-value 形式
	for i := 0; i+1 < len(kvs); i += 2 {
		attrs = append(attrs, toAttr(fmt.Sprint(kvs[i]), kvs[i+1]))
	}
	return attrs
}

func toAttr(key string, v any) attribute.KeyValue {
	switch x := v.(type) {
	case string:
		return attribute.String(key, x)
	case bool:
		return attribute.Bool(key, x)
	case int:
		return attribute.Int(key, x)
	case int64:
		return attribute.Int64(key, x)
	case float64:
		return attribute.Float64(key, x)
	case time.Duration:
		return attribute.String(key, x.String())
	case error:
		return attribute.String(key, x.Error())
	default:
		return attribute.String(key, fmt.Sprintf("%v", x))
	}
}

func toError(v any) error {
	switch x := v.(type) {
	case error:
		return x
	case nil:
		return errors.New("<nil>")
	default:
		return errors.New(fmt.Sprint(x))
	}
}
