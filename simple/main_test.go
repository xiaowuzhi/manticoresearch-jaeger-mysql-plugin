package main

import (
	"context"
	"testing"

	"github.com/opentracing/opentracing-go"
	"github.com/opentracing/opentracing-go/mocktracer"
	"github.com/stretchr/testify/assert"
)

// TestHelloWorld 测试 HelloWorld 函数
func TestHelloWorld(t *testing.T) {
	// 使用 mock tracer
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	// 创建 span 和 context
	span := tracer.StartSpan("test")
	ctx := opentracing.ContextWithSpan(context.Background(), span)

	// 测试
	result := HelloWorld(ctx, "World")
	
	assert.Equal(t, "Hello, World!", result)
	
	span.Finish()

	// 验证 span 创建
	spans := tracer.FinishedSpans()
	assert.True(t, len(spans) > 0, "应该创建了 span")
}

// TestHelloWorldWithDifferentNames 测试不同名字
func TestHelloWorldWithDifferentNames(t *testing.T) {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	testCases := []struct {
		name     string
		input    string
		expected string
	}{
		{"中文名字", "张三", "Hello, 张三!"},
		{"英文名字", "John", "Hello, John!"},
		{"空字符串", "", "Hello, !"},
		{"特殊字符", "@#$", "Hello, @#$!"},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			span := tracer.StartSpan("test-" + tc.name)
			ctx := opentracing.ContextWithSpan(context.Background(), span)
			
			result := HelloWorld(ctx, tc.input)
			
			assert.Equal(t, tc.expected, result)
			span.Finish()
		})
	}
}

// TestAdd 测试加法函数
func TestAdd(t *testing.T) {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	testCases := []struct {
		name     string
		a, b     int
		expected int
	}{
		{"正数相加", 10, 20, 30},
		{"负数相加", -5, -3, -8},
		{"正负数相加", 10, -5, 5},
		{"零", 0, 0, 0},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			span := tracer.StartSpan("test-add")
			ctx := opentracing.ContextWithSpan(context.Background(), span)
			
			result := Add(ctx, tc.a, tc.b)
			
			assert.Equal(t, tc.expected, result)
			span.Finish()
		})
	}
}

// TestProcessOrder 测试订单处理
func TestProcessOrder(t *testing.T) {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	span := tracer.StartSpan("test")
	ctx := opentracing.ContextWithSpan(context.Background(), span)

	result := ProcessOrder(ctx, "TEST-001")
	
	assert.Contains(t, result, "TEST-001")
	assert.Contains(t, result, "processed successfully")
	
	span.Finish()

	// 验证多层 span 创建
	spans := tracer.FinishedSpans()
	assert.True(t, len(spans) >= 4, "应该创建了多个 span（ProcessOrder + 子函数）")

	// 验证 span 名称
	spanNames := make(map[string]bool)
	for _, s := range spans {
		spanNames[s.OperationName] = true
	}
	
	assert.True(t, spanNames["ProcessOrder"], "应该有 ProcessOrder span")
	assert.True(t, spanNames["validateOrder"], "应该有 validateOrder span")
	assert.True(t, spanNames["calculateAmount"], "应该有 calculateAmount span")
	assert.True(t, spanNames["saveOrder"], "应该有 saveOrder span")
}

// TestValidateOrder 测试订单验证
func TestValidateOrder(t *testing.T) {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	span := tracer.StartSpan("test")
	ctx := opentracing.ContextWithSpan(context.Background(), span)

	result := validateOrder(ctx, "ORDER-001")
	
	assert.True(t, result)
	span.Finish()
}

// TestCalculateAmount 测试金额计算
func TestCalculateAmount(t *testing.T) {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	span := tracer.StartSpan("test")
	ctx := opentracing.ContextWithSpan(context.Background(), span)

	amount := calculateAmount(ctx, "ORDER-001")
	
	assert.Equal(t, 99.99, amount)
	span.Finish()
}

// TestSaveOrder 测试保存订单
func TestSaveOrder(t *testing.T) {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	span := tracer.StartSpan("test")
	ctx := opentracing.ContextWithSpan(context.Background(), span)

	// 不应该 panic
	assert.NotPanics(t, func() {
		saveOrder(ctx, "ORDER-001", 99.99)
	})
	
	span.Finish()
}

// TestInitJaeger 测试 Jaeger 初始化
func TestInitJaeger(t *testing.T) {
	tracer, closer := InitJaeger("test-service")
	defer closer()

	assert.NotNil(t, tracer)
}

// TestSpanTagsAndLogs 测试 Span 的 Tags 和 Logs
func TestSpanTagsAndLogs(t *testing.T) {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	span := tracer.StartSpan("test")
	ctx := opentracing.ContextWithSpan(context.Background(), span)

	HelloWorld(ctx, "TestUser")
	span.Finish()

	spans := tracer.FinishedSpans()
	assert.True(t, len(spans) > 0)

	// 找到 HelloWorld span
	var helloSpan *mocktracer.MockSpan
	for _, s := range spans {
		if s.OperationName == "HelloWorld" {
			helloSpan = s
			break
		}
	}

	assert.NotNil(t, helloSpan)
	
	// 验证 tag
	tags := helloSpan.Tags()
	assert.Equal(t, "TestUser", tags["name"])
	
	// 验证 logs
	logs := helloSpan.Logs()
	assert.True(t, len(logs) > 0)
}

// BenchmarkHelloWorld Benchmark 测试
func BenchmarkHelloWorld(b *testing.B) {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	span := tracer.StartSpan("benchmark")
	ctx := opentracing.ContextWithSpan(context.Background(), span)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		HelloWorld(ctx, "Benchmark")
	}
}

// BenchmarkAdd Benchmark 测试
func BenchmarkAdd(b *testing.B) {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	span := tracer.StartSpan("benchmark")
	ctx := opentracing.ContextWithSpan(context.Background(), span)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		Add(ctx, 10, 20)
	}
}

// BenchmarkProcessOrder Benchmark 测试
func BenchmarkProcessOrder(b *testing.B) {
	tracer := mocktracer.New()
	opentracing.SetGlobalTracer(tracer)

	span := tracer.StartSpan("benchmark")
	ctx := opentracing.ContextWithSpan(context.Background(), span)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		ProcessOrder(ctx, "BENCH-001")
	}
}

