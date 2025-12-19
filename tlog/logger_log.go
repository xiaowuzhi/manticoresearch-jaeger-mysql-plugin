package tlog

import (
	"context"
	"errors"
	"fmt"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// Info 记录普通信息
func (l *Logger) Info(ctx context.Context, msg any, kvs ...any) {
	zl := l.z(ctx)
	zFields((&zl).Info(), kvs...).Msg(fmt.Sprint(msg))

	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return
	}
	attrs := l.buildAttrs(fmt.Sprint(msg), kvs...)
	span.AddEvent(eventInfo, trace.WithAttributes(attrs...))
}

// Error 记录错误
func (l *Logger) Error(ctx context.Context, err any, msg ...any) {
	zl := l.z(ctx)
	ev := (&zl).Error()
	switch v := err.(type) {
	case nil:
		ev = ev.Str("err", "nil")
	case error:
		ev = ev.Err(v)
	default:
		ev = ev.Interface("err", v)
	}
	zFields(ev, msg...).Msg(fmt.Sprint(err))

	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return
	}

	var e error
	switch v := err.(type) {
	case nil:
		e = errors.New("nil")
	case error:
		e = v
	default:
		e = errors.New(fmt.Sprint(v))
	}

	span.RecordError(e)
	span.SetStatus(codes.Error, e.Error())

	attrs := []attribute.KeyValue{
		attribute.String("error", e.Error()),
	}
	attrs = append(attrs, l.buildKVs(msg...)...)
	span.AddEvent(eventError, trace.WithAttributes(attrs...))
}

// Errorf 格式化记录错误
func (l *Logger) Errorf(ctx context.Context, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	l.Error(ctx, msg)
}

// Warn 记录警告
func (l *Logger) Warn(ctx context.Context, msg any, kvs ...any) {
	zl := l.z(ctx)
	zFields((&zl).Warn(), kvs...).Msg(fmt.Sprint(msg))

	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return
	}
	attrs := l.buildAttrs(fmt.Sprint(msg), kvs...)
	span.AddEvent(eventWarn, trace.WithAttributes(attrs...))
}

// Debug 记录调试信息
func (l *Logger) Debug(ctx context.Context, msg any, data ...any) {
	zl := l.z(ctx)
	zFields((&zl).Debug(), data...).Msg(fmt.Sprint(msg))

	span := trace.SpanFromContext(ctx)
	sc := span.SpanContext()
	if !sc.IsValid() {
		return
	}
	attrs := []attribute.KeyValue{
		attribute.String("msg", fmt.Sprint(msg)),
	}
	for i, v := range data {
		attrs = append(attrs, attribute.String(fmt.Sprintf("arg_%d", i), fmt.Sprintf("%+v", v)))
	}
	span.AddEvent(eventDebug, trace.WithAttributes(attrs...))
}


