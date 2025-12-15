package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.21.0"
	"go.opentelemetry.io/otel/trace"
)

const (
	// OpenTelemetry 批处理配置环境变量
	OTEL_BSP_EXPORT_TIMEOUT        = "OTEL_BSP_EXPORT_TIMEOUT"
	OTEL_BSP_MAX_QUEUE_SIZE        = "OTEL_BSP_MAX_QUEUE_SIZE"
	OTEL_BSP_MAX_EXPORT_BATCH_SIZE = "OTEL_BSP_MAX_EXPORT_BATCH_SIZE"
)

// OTELConfig OTLP 配置
type OTELConfig struct {
	ServiceName string // 服务名称
	Endpoint    string // OTLP Collector 地址
	Token       string // 认证 Token (可选)
	HostName    string // 主机名
}

// NewOTELTrace 初始化 OpenTelemetry Tracer
// 支持 OTLP gRPC 协议连接到 Jaeger Collector
func NewOTELTrace(config OTELConfig) (trace.Tracer, func(), error) {
	// 设置批处理参数
	os.Setenv(OTEL_BSP_MAX_QUEUE_SIZE, "30")
	os.Setenv(OTEL_BSP_MAX_EXPORT_BATCH_SIZE, "10")

	// 默认值
	if config.HostName == "" {
		config.HostName = "localhost"
	}
	if config.Endpoint == "" {
		config.Endpoint = "localhost:4317"
	}

	// 创建资源（Resource）对象
	attrs := []attribute.KeyValue{
		semconv.ServiceName(config.ServiceName),
		semconv.HostName(config.HostName),
	}
	if config.Token != "" {
		attrs = append(attrs, attribute.String("token", config.Token))
	}

	resources, resourcesErr := resource.New(context.Background(),
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithTelemetrySDK(),
		resource.WithHost(),
		resource.WithAttributes(attrs...),
	)

	if resourcesErr != nil {
		return nil, nil, fmt.Errorf("failed to create resources: %w", resourcesErr)
	}

	// 创建 OTLP gRPC 导出器（Exporter）
	exporter, exporterErr := otlptracegrpc.New(context.Background(),
		otlptracegrpc.WithInsecure(), // K8s 内部通信不需要 TLS
		otlptracegrpc.WithEndpoint(config.Endpoint),
	)

	if exporterErr != nil {
		return nil, nil, fmt.Errorf("failed to create exporter: %w", exporterErr)
	}

	// 创建 TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()), // 100% 采样
		sdktrace.WithResource(resources),
		sdktrace.WithBatcher(exporter),
	)

	// 设置全局 TracerProvider
	otel.SetTracerProvider(tp)

	// 创建 Tracer
	tracer := otel.Tracer(config.ServiceName)

	log.Printf("✓ OTLP Tracer 初始化成功: %s (Endpoint: %s)", config.ServiceName, config.Endpoint)

	// 返回清理函数
	cleanup := func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("Error shutting down tracer provider: %v", err)
		}
	}

	return tracer, cleanup, nil
}

// GetOTELEndpoint 根据环境变量获取 OTLP Endpoint
// 支持 K8s 和本地环境
func GetOTELEndpoint() string {
	// 优先使用环境变量
	if endpoint := os.Getenv("OTEL_EXPORTER_OTLP_ENDPOINT"); endpoint != "" {
		return endpoint
	}

	// K8s 内部地址
	if os.Getenv("KUBERNETES_SERVICE_HOST") != "" {
		return "jaeger-collector.tracing.svc.cluster.local:4317"
	}

	// 本地开发
	return "localhost:4317"
}

// CreateSpan 创建一个简单的 Span
func CreateSpan(ctx context.Context, tracer trace.Tracer, name string) (context.Context, trace.Span) {
	return tracer.Start(ctx, name)
}

// AddSpanAttributes 添加 Span 属性
func AddSpanAttributes(span trace.Span, attrs map[string]interface{}) {
	for key, value := range attrs {
		switch v := value.(type) {
		case string:
			span.SetAttributes(attribute.String(key, v))
		case int:
			span.SetAttributes(attribute.Int(key, v))
		case int64:
			span.SetAttributes(attribute.Int64(key, v))
		case float64:
			span.SetAttributes(attribute.Float64(key, v))
		case bool:
			span.SetAttributes(attribute.Bool(key, v))
		default:
			span.SetAttributes(attribute.String(key, fmt.Sprintf("%v", v)))
		}
	}
}

// AddSpanEvent 添加 Span 事件
func AddSpanEvent(span trace.Span, name string, attrs map[string]string) {
	eventAttrs := make([]attribute.KeyValue, 0, len(attrs))
	for k, v := range attrs {
		eventAttrs = append(eventAttrs, attribute.String(k, v))
	}
	span.AddEvent(name, trace.WithAttributes(eventAttrs...))
}

// RecordError 记录错误到 Span
func RecordError(span trace.Span, err error) {
	span.RecordError(err)
	span.SetAttributes(attribute.Bool("error", true))
}

