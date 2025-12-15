package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/opentracing/opentracing-go"
	"github.com/uber/jaeger-client-go/config"
)

// InitJaeger 初始化 Jaeger Tracer
// 支持环境变量:
//
//	JAEGER_HOST - Jaeger Agent 主机 (默认: localhost)
//	JAEGER_PORT - Jaeger Agent 端口 (默认: 6831)
func InitJaeger(serviceName string) (opentracing.Tracer, func()) {
	// 从环境变量读取配置
	jaegerHost := os.Getenv("JAEGER_HOST")
	if jaegerHost == "" {
		jaegerHost = "localhost"
	}

	jaegerPort := os.Getenv("JAEGER_PORT")
	if jaegerPort == "" {
		jaegerPort = "6831"
	}

	agentHostPort := fmt.Sprintf("%s:%s", jaegerHost, jaegerPort)

	cfg := &config.Configuration{
		ServiceName: serviceName,
		Sampler: &config.SamplerConfig{
			Type:  "const",
			Param: 1, // 100% 采样
		},
		Reporter: &config.ReporterConfig{
			LogSpans:           true,
			LocalAgentHostPort: agentHostPort,
		},
	}

	tracer, closer, err := cfg.NewTracer()
	if err != nil {
		log.Printf("无法初始化 Jaeger: %v", err)
		// 返回 NoopTracer
		return opentracing.NoopTracer{}, func() {}
	}

	opentracing.SetGlobalTracer(tracer)
	log.Printf("Jaeger Tracer 初始化成功: %s (Agent: %s)", serviceName, agentHostPort)

	return tracer, func() {
		if err := closer.Close(); err != nil {
			log.Printf("关闭 Jaeger Tracer 失败: %v", err)
		}
	}
}

// HelloWorld 简单的 Hello World 函数，带追踪
func HelloWorld(ctx context.Context, name string) string {
	span, ctx := opentracing.StartSpanFromContext(ctx, "HelloWorld")
	defer span.Finish()

	span.SetTag("name", name)
	span.LogKV("event", "saying hello", "name", name)

	// 模拟一些处理时间
	time.Sleep(10 * time.Millisecond)

	message := fmt.Sprintf("Hello, %s!", name)
	span.LogKV("event", "message generated", "message", message)

	return message
}

// Add 简单的加法函数，带追踪
func Add(ctx context.Context, a, b int) int {
	span, ctx := opentracing.StartSpanFromContext(ctx, "Add")
	defer span.Finish()

	span.SetTag("a", a)
	span.SetTag("b", b)

	result := a + b
	span.SetTag("result", result)
	span.LogKV("event", "calculation done", "result", result)

	return result
}

// ProcessOrder 模拟处理订单，带多层追踪
func ProcessOrder(ctx context.Context, orderID string) string {
	span, ctx := opentracing.StartSpanFromContext(ctx, "ProcessOrder")
	defer span.Finish()

	span.SetTag("orderID", orderID)
	span.LogKV("event", "order processing started")

	// 第一步：验证订单
	validateOrder(ctx, orderID)

	// 第二步：计算金额
	amount := calculateAmount(ctx, orderID)

	// 第三步：保存订单
	saveOrder(ctx, orderID, amount)

	span.LogKV("event", "order processing completed", "status", "success")

	return fmt.Sprintf("Order %s processed successfully", orderID)
}

func validateOrder(ctx context.Context, orderID string) bool {
	span, _ := opentracing.StartSpanFromContext(ctx, "validateOrder")
	defer span.Finish()

	span.SetTag("orderID", orderID)
	time.Sleep(20 * time.Millisecond)

	span.LogKV("event", "validation completed", "valid", true)
	return true
}

func calculateAmount(ctx context.Context, orderID string) float64 {
	span, _ := opentracing.StartSpanFromContext(ctx, "calculateAmount")
	defer span.Finish()

	span.SetTag("orderID", orderID)
	time.Sleep(30 * time.Millisecond)

	amount := 99.99
	span.SetTag("amount", amount)
	span.LogKV("event", "amount calculated", "amount", amount)

	return amount
}

func saveOrder(ctx context.Context, orderID string, amount float64) {
	span, _ := opentracing.StartSpanFromContext(ctx, "saveOrder")
	defer span.Finish()

	span.SetTag("orderID", orderID)
	span.SetTag("amount", amount)
	time.Sleep(40 * time.Millisecond)

	span.LogKV("event", "order saved", "status", "success")
}

func main() {
	// 初始化 Jaeger
	tracer, closer := InitJaeger("simple-demo")
	defer closer()

	// 示例 1: Hello World
	fmt.Println("\n=== 示例 1: Hello World ===")
	span1 := tracer.StartSpan("main-example1")
	ctx1 := opentracing.ContextWithSpan(context.Background(), span1)
	result1 := HelloWorld(ctx1, "张三")
	fmt.Println(result1)
	span1.Finish()

	// 示例 2: 简单计算
	fmt.Println("\n=== 示例 2: 简单计算 ===")
	span2 := tracer.StartSpan("main-example2")
	ctx2 := opentracing.ContextWithSpan(context.Background(), span2)
	result2 := Add(ctx2, 10, 20)
	fmt.Printf("10 + 20 = %d\n", result2)
	span2.Finish()

	// 示例 3: 多层调用
	fmt.Println("\n=== 示例 3: 处理订单（多层调用）===")
	span3 := tracer.StartSpan("main-example3")
	ctx3 := opentracing.ContextWithSpan(context.Background(), span3)
	result3 := ProcessOrder(ctx3, "ORDER-001")
	fmt.Println(result3)
	span3.Finish()

	// 等待一下，确保数据上报
	fmt.Println("\n等待数据上报...")
	time.Sleep(2 * time.Second)

	fmt.Println("\n✓ 完成！请访问 Jaeger UI 查看追踪数据")
	fmt.Println("  http://localhost:16686")
}
