package tlog

import (
	"fmt"

	"github.com/rs/zerolog"
)

func zFields(e *zerolog.Event, kvs ...any) *zerolog.Event {
	if e == nil || len(kvs) == 0 {
		return e
	}

	// map 形式
	if len(kvs) == 1 {
		if m, ok := kvs[0].(map[string]any); ok {
			for k, v := range m {
				e = e.Interface(k, v)
			}
			return e
		}
	}

	// key-value 形式；不成对则容错
	for i := 0; i < len(kvs); i++ {
		if i+1 >= len(kvs) {
			e = e.Interface(fmt.Sprintf("arg_%d", i), kvs[i])
			break
		}
		key := fmt.Sprint(kvs[i])
		val := kvs[i+1]
		e = e.Interface(key, val)
		i++
	}
	return e
}


