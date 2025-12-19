package tlog

import (
	"fmt"
	"time"

	"go.opentelemetry.io/otel/attribute"
)

func (l *Logger) buildAttrs(msg string, kvs ...interface{}) []attribute.KeyValue {
	attrs := []attribute.KeyValue{
		attribute.String("msg", msg),
	}
	attrs = append(attrs, l.buildKVs(kvs...)...)
	return attrs
}

func (l *Logger) buildKVs(kvs ...any) []attribute.KeyValue {
	out := make([]attribute.KeyValue, 0, len(kvs))
	// map 形式
	if len(kvs) == 1 {
		if m, ok := kvs[0].(map[string]any); ok {
			for k, v := range m {
				out = append(out, toAttr(k, v))
			}
			return out
		}
	}

	// key-value 形式；不成对则容错
	for i := 0; i < len(kvs); i++ {
		if i+1 >= len(kvs) {
			out = append(out, toAttr(fmt.Sprintf("arg_%d", i), kvs[i]))
			break
		}
		key := fmt.Sprint(kvs[i])
		val := kvs[i+1]
		out = append(out, toAttr(key, val))
		i++
	}
	return out
}

func toAttr(key string, v any) attribute.KeyValue {
	switch x := v.(type) {
	case string:
		return attribute.String(key, x)
	case []byte:
		return attribute.String(key, string(x))
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
		return attribute.String(key, fmt.Sprintf("%+v", x))
	}
}


