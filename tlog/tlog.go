// Package tlog 提供基于 OpenTelemetry 的统一日志/追踪库
//
// 使用示例:
//
//	tlog.Init(tlog.Config{ServiceName: "my-service", Endpoint: "localhost:4317"})
//	defer tlog.Shutdown(context.Background())
//
//	ctx, span := tlog.Log.Start(ctx, "operation")
//	defer tlog.Log.End(span)
//
//	tlog.Log.Info(ctx, "消息", "key", "value")
//	tlog.Log.Error(ctx, err)
package tlog

import (
	"context"
	"fmt"
	"io"
	"os"
	"time"

	"github.com/rs/zerolog"
	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.22.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// ============================================================
// 配置
// ============================================================

// Config 初始化配置
type Config struct {
	ServiceName    string        // 服务名称（必填）
	Endpoint       string        // OTLP gRPC 端点，默认 localhost:4317
	HostName       string        // 主机名，默认自动获取
	ConnectTimeout time.Duration // 连接超时，默认 1s
	CheckConn      *bool         // 是否检测连接，默认 true
	FailFast       *bool         // 连接失败是否直接报错，默认 false

	// 可选：自定义 zerolog
	ZLogger *zerolog.Logger
	ZWriter io.Writer
	ZLevel  zerolog.Level
}

// ============================================================
// 全局实例
// ============================================================

// Log 全局日志实例
var Log = &Logger{}

// Logger 日志器
type Logger struct {
	provider *sdktrace.TracerProvider
	tracer   trace.Tracer
	zlog     zerolog.Logger
}

// ============================================================
// 初始化 / 关闭
// ============================================================

// Init 初始化 tlog
func Init(cfg Config) error {
	applyDefaults(&cfg)
	initZerolog(&cfg)

	Log.zlog.Info().
		Str("service", cfg.ServiceName).
		Str("endpoint", cfg.Endpoint).
		Msg("tlog init")

	// 创建 Resource
	res, err := resource.New(context.Background(),
		resource.WithFromEnv(),
		resource.WithProcess(),
		resource.WithHost(),
		resource.WithAttributes(
			semconv.ServiceNameKey.String(cfg.ServiceName),
			semconv.HostNameKey.String(cfg.HostName),
		),
	)
	if err != nil {
		return fmt.Errorf("create resource: %w", err)
	}

	// 创建 Exporter
	exporter, err := otlptracegrpc.New(context.Background(),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	)
	if err != nil {
		return fmt.Errorf("create exporter: %w", err)
	}

	// 创建 TracerProvider
	Log.provider = sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
	)
	otel.SetTracerProvider(Log.provider)
	Log.tracer = otel.Tracer(cfg.ServiceName)

	// 连接探测
	if boolVal(cfg.CheckConn, true) {
		if ok, err := probeConn(cfg.Endpoint, cfg.ConnectTimeout); ok {
			Log.zlog.Info().Str("endpoint", cfg.Endpoint).Msg("endpoint ok")
		} else {
			Log.zlog.Warn().Str("endpoint", cfg.Endpoint).Err(err).Msg("endpoint unreachable")
			if boolVal(cfg.FailFast, false) {
				return fmt.Errorf("connect failed: %w", err)
			}
		}
	}

	return nil
}

// Shutdown 关闭 tlog
func Shutdown(ctx context.Context) error {
	if Log.provider != nil {
		return Log.provider.Shutdown(ctx)
	}
	return nil
}

// ============================================================
// 辅助函数
// ============================================================

func applyDefaults(cfg *Config) {
	if cfg.ServiceName == "" {
		cfg.ServiceName = "default-service"
	}
	if cfg.Endpoint == "" {
		cfg.Endpoint = "localhost:4317"
	}
	if cfg.HostName == "" {
		cfg.HostName, _ = os.Hostname()
	}
	if cfg.ConnectTimeout <= 0 {
		cfg.ConnectTimeout = time.Second
	}
	// OTEL 批处理配置
	setEnvDefault("OTEL_BSP_MAX_QUEUE_SIZE", "30")
	setEnvDefault("OTEL_BSP_MAX_EXPORT_BATCH_SIZE", "1")
}

func initZerolog(cfg *Config) {
	zerolog.TimeFieldFormat = time.RFC3339Nano

	if cfg.ZLogger != nil {
		Log.zlog = *cfg.ZLogger
		return
	}

	w := cfg.ZWriter
	if w == nil {
		w = os.Stdout
	}

	level := cfg.ZLevel
	if level == zerolog.NoLevel {
		level = zerolog.InfoLevel
	}

	Log.zlog = zerolog.New(w).Level(level).With().
		Timestamp().
		Str("service", cfg.ServiceName).
		Logger()
}

func probeConn(endpoint string, timeout time.Duration) (bool, error) {
	ctx, cancel := context.WithTimeout(context.Background(), timeout)
	defer cancel()
	conn, err := grpc.DialContext(ctx, endpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return false, err
	}
	conn.Close()
	return true, nil
}

func boolVal(ptr *bool, def bool) bool {
	if ptr == nil {
		return def
	}
	return *ptr
}

func setEnvDefault(key, val string) {
	if os.Getenv(key) == "" {
		os.Setenv(key, val)
	}
}

// TraceID 获取当前 trace_id
func TraceID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if sc := span.SpanContext(); sc.IsValid() {
		return sc.TraceID().String()
	}
	return ""
}

// SpanID 获取当前 span_id
func SpanID(ctx context.Context) string {
	span := trace.SpanFromContext(ctx)
	if sc := span.SpanContext(); sc.IsValid() {
		return sc.SpanID().String()
	}
	return ""
}
