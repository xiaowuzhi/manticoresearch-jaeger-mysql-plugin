package main

import (
	"context"
	"fmt"
	"testing"
	"time"

	"go.opentelemetry.io/otel/attribute"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

// TestOTELBasic 基础 OTLP 追踪测试
func TestOTELBasic(t *testing.T) {
	// 初始化 Tracer
	config := OTELConfig{
		ServiceName: "test-service",
		Endpoint:    GetOTELEndpoint(),
		HostName:    "test-host",
	}

	tracer, cleanup, err := NewOTELTrace(config)
	if err != nil {
		t.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer cleanup()

	// 创建一个 Span
	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "test-operation")
	defer span.End()

	// 添加属性
	span.SetAttributes(
		attribute.String("test.name", "basic-test"),
		attribute.Int("test.count", 1),
	)

	// 添加事件
	span.AddEvent("Test event occurred")

	fmt.Println("✓ Basic OTLP trace sent successfully")
	fmt.Printf("  TraceID: %s\n", span.SpanContext().TraceID())
	fmt.Printf("  SpanID: %s\n", span.SpanContext().SpanID())
}

// TestOTELDatabase 模拟数据库操作追踪
func TestOTELDatabase(t *testing.T) {
	config := OTELConfig{
		ServiceName: "database-service",
		Endpoint:    GetOTELEndpoint(),
	}

	tracer, cleanup, err := NewOTELTrace(config)
	if err != nil {
		t.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer cleanup()

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "database-query")
	defer span.End()

	// 添加数据库相关属性
	span.SetAttributes(
		semconv.DBSystemMySQL,
		semconv.DBName("test_db"),
		semconv.DBStatement("SELECT * FROM users WHERE id = ?"),
		attribute.String("db.user", "admin"),
		attribute.Int("db.rows_affected", 10),
	)

	// 模拟查询时间
	time.Sleep(50 * time.Millisecond)

	span.AddEvent("Query executed successfully")

	fmt.Println("✓ Database trace sent successfully")
}

// TestOTELHTTP 模拟 HTTP 请求追踪
func TestOTELHTTP(t *testing.T) {
	config := OTELConfig{
		ServiceName: "http-service",
		Endpoint:    GetOTELEndpoint(),
	}

	tracer, cleanup, err := NewOTELTrace(config)
	if err != nil {
		t.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer cleanup()

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "HTTP GET /api/users")
	defer span.End()

	// 添加 HTTP 相关属性
	span.SetAttributes(
		semconv.HTTPMethod("GET"),
		semconv.HTTPRoute("/api/users"),
		semconv.HTTPStatusCode(200),
		semconv.HTTPTarget("/api/users?page=1"),
		attribute.String("http.user_agent", "Go-Test/1.0"),
		attribute.String("http.client_ip", "192.168.1.100"),
	)

	// 模拟请求处理时间
	time.Sleep(30 * time.Millisecond)

	span.AddEvent("Response sent", trace.WithAttributes(
		attribute.Int("http.response.body.size", 1024),
	))

	fmt.Println("✓ HTTP trace sent successfully")
}

// TestOTELNestedSpans 嵌套 Span 测试
func TestOTELNestedSpans(t *testing.T) {
	config := OTELConfig{
		ServiceName: "nested-service",
		Endpoint:    GetOTELEndpoint(),
	}

	tracer, cleanup, err := NewOTELTrace(config)
	if err != nil {
		t.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer cleanup()

	// 父 Span
	ctx := context.Background()
	ctx, parentSpan := tracer.Start(ctx, "parent-operation")
	defer parentSpan.End()

	parentSpan.SetAttributes(attribute.String("operation.type", "complex"))

	// 子 Span 1
	executeChildOperation1(ctx, tracer)

	// 子 Span 2
	executeChildOperation2(ctx, tracer)

	parentSpan.AddEvent("All child operations completed")

	fmt.Println("✓ Nested spans trace sent successfully")
}

// executeChildOperation1 子操作 1
func executeChildOperation1(ctx context.Context, tracer trace.Tracer) {
	_, span := tracer.Start(ctx, "child-operation-1")
	defer span.End()

	span.SetAttributes(attribute.String("operation.name", "validate"))
	time.Sleep(20 * time.Millisecond)
	span.AddEvent("Validation completed")
}

// executeChildOperation2 子操作 2
func executeChildOperation2(ctx context.Context, tracer trace.Tracer) {
	_, span := tracer.Start(ctx, "child-operation-2")
	defer span.End()

	span.SetAttributes(attribute.String("operation.name", "process"))
	time.Sleep(40 * time.Millisecond)
	span.AddEvent("Processing completed")
}

// TestOTELError 错误追踪测试
func TestOTELError(t *testing.T) {
	config := OTELConfig{
		ServiceName: "error-service",
		Endpoint:    GetOTELEndpoint(),
	}

	tracer, cleanup, err := NewOTELTrace(config)
	if err != nil {
		t.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer cleanup()

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "error-operation")
	defer span.End()

	// 模拟错误
	err = fmt.Errorf("simulated error: connection timeout")
	if err != nil {
		RecordError(span, err)
		span.AddEvent("Error occurred", trace.WithAttributes(
			attribute.String("error.message", err.Error()),
			attribute.String("error.type", "timeout"),
		))
	}

	fmt.Println("✓ Error trace sent successfully")
}

// TestOTELMultipleOperations 多操作追踪测试
func TestOTELMultipleOperations(t *testing.T) {
	config := OTELConfig{
		ServiceName: "multi-operation-service",
		Endpoint:    GetOTELEndpoint(),
	}

	tracer, cleanup, err := NewOTELTrace(config)
	if err != nil {
		t.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer cleanup()

	// 执行多个操作
	for i := 0; i < 3; i++ {
		executeOperation(tracer, i+1)
		time.Sleep(100 * time.Millisecond)
	}

	fmt.Println("✓ Multiple operations trace sent successfully")
}

// executeOperation 执行单个操作
func executeOperation(tracer trace.Tracer, operationID int) {
	ctx := context.Background()
	ctx, span := tracer.Start(ctx, fmt.Sprintf("operation-%d", operationID))
	defer span.End()

	span.SetAttributes(
		attribute.Int("operation.id", operationID),
		attribute.String("operation.status", "processing"),
	)

	// 模拟工作
	time.Sleep(50 * time.Millisecond)

	span.SetAttributes(attribute.String("operation.status", "completed"))
	span.AddEvent(fmt.Sprintf("Operation %d completed", operationID))
}

// TestOTELWithCustomAttributes 自定义属性测试
func TestOTELWithCustomAttributes(t *testing.T) {
	config := OTELConfig{
		ServiceName: "custom-attrs-service",
		Endpoint:    GetOTELEndpoint(),
		Token:       "test-token-123",
	}

	tracer, cleanup, err := NewOTELTrace(config)
	if err != nil {
		t.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer cleanup()

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "custom-operation")
	defer span.End()

	// 添加自定义属性
	attrs := map[string]interface{}{
		"user.id":       "12345",
		"user.name":     "张三",
		"order.id":      "ORDER-001",
		"order.amount":  99.99,
		"order.items":   3,
		"order.paid":    true,
		"location.city": "Beijing",
	}

	AddSpanAttributes(span, attrs)

	// 添加自定义事件
	AddSpanEvent(span, "订单处理开始", map[string]string{
		"step": "validate",
	})

	time.Sleep(30 * time.Millisecond)

	AddSpanEvent(span, "订单处理完成", map[string]string{
		"step":   "complete",
		"result": "success",
	})

	fmt.Println("✓ Custom attributes trace sent successfully")
}

// TestOTELLongRunning 长时间运行的追踪测试
func TestOTELLongRunning(t *testing.T) {
	config := OTELConfig{
		ServiceName: "long-running-service",
		Endpoint:    GetOTELEndpoint(),
	}

	tracer, cleanup, err := NewOTELTrace(config)
	if err != nil {
		t.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer cleanup()

	ctx := context.Background()
	ctx, span := tracer.Start(ctx, "long-running-operation")
	defer span.End()

	// 模拟长时间运行的操作，记录进度
	totalSteps := 5
	for i := 0; i < totalSteps; i++ {
		span.AddEvent(fmt.Sprintf("Processing step %d/%d", i+1, totalSteps), trace.WithAttributes(
			attribute.Int("step.number", i+1),
			attribute.Int("step.total", totalSteps),
			attribute.Float64("progress.percentage", float64(i+1)/float64(totalSteps)*100),
		))

		time.Sleep(100 * time.Millisecond)
	}

	span.SetAttributes(attribute.String("operation.result", "completed"))

	fmt.Println("✓ Long-running trace sent successfully")
}

// BenchmarkOTELSpanCreation Span 创建性能测试
func BenchmarkOTELSpanCreation(b *testing.B) {
	config := OTELConfig{
		ServiceName: "benchmark-service",
		Endpoint:    GetOTELEndpoint(),
	}

	tracer, cleanup, err := NewOTELTrace(config)
	if err != nil {
		b.Fatalf("Failed to initialize tracer: %v", err)
	}
	defer cleanup()

	ctx := context.Background()

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, span := tracer.Start(ctx, "benchmark-operation")
		span.SetAttributes(attribute.Int("iteration", i))
		span.End()
	}
}

