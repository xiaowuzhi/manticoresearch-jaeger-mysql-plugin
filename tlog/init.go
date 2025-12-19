package tlog

import (
	"context"
	"fmt"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.22.0"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

// Init 初始化全局 Logger
func Init(cfg Config) error {
	applyDefaults(&cfg)
	initZerolog(cfg)

	Log.zlog.Info().
		Str("service", cfg.ServiceName).
		Str("endpoint", cfg.Endpoint).
		Str("host", cfg.HostName).
		Bool("check_conn", cfgCheckConn(cfg)).
		Dur("timeout", cfg.ConnectTimeout).
		Msg("tlog init")

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
		return fmt.Errorf("create resource failed: %w", err)
	}

	exporter, err := otlptracegrpc.New(context.Background(),
		otlptracegrpc.WithInsecure(),
		otlptracegrpc.WithEndpoint(cfg.Endpoint),
	)
	if err != nil {
		return fmt.Errorf("create exporter failed: %w", err)
	}

	Log.provider = sdktrace.NewTracerProvider(
		sdktrace.WithSampler(sdktrace.AlwaysSample()),
		sdktrace.WithResource(res),
		sdktrace.WithBatcher(exporter),
	)

	otel.SetTracerProvider(Log.provider)
	Log.tracer = otel.Tracer(cfg.ServiceName)

	// 注意：连通性探测放在 tracer 初始化之后，这样即使 endpoint 不可用也能正常产生 trace_id/span_id
	if cfgCheckConn(cfg) {
		ok, err := probeConn(cfg.Endpoint, cfg.ConnectTimeout)
		if ok {
			Log.zlog.Info().Str("endpoint", cfg.Endpoint).Msg("otel endpoint probe ok")
		} else {
			Log.zlog.Warn().Str("endpoint", cfg.Endpoint).Err(err).Msg("otel endpoint probe failed")
			if cfgFailFast(cfg) {
				return fmt.Errorf("connect otlp endpoint failed (%s): %w", cfg.Endpoint, err)
			}
		}
	}

	return nil
}

// Shutdown 关闭
func Shutdown(ctx context.Context) error {
	if Log.provider != nil {
		return Log.provider.Shutdown(ctx)
	}
	return nil
}

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
		cfg.ConnectTimeout = 1 * time.Second
	}

	// 参考 jaegerv1/comm_jaeger.go：通过 env 调整 BatchSpanProcessor 队列/批次参数
	// 这里采用“如果用户没设置，则给默认值”的方式，避免覆盖调用方已有配置。
	if os.Getenv("OTEL_BSP_MAX_QUEUE_SIZE") == "" {
		_ = os.Setenv("OTEL_BSP_MAX_QUEUE_SIZE", "30")
	}
	if os.Getenv("OTEL_BSP_MAX_EXPORT_BATCH_SIZE") == "" {
		_ = os.Setenv("OTEL_BSP_MAX_EXPORT_BATCH_SIZE", "1")
	}
}

func cfgCheckConn(cfg Config) bool {
	if cfg.CheckConn == nil {
		return true
	}
	return *cfg.CheckConn
}

func cfgFailFast(cfg Config) bool {
	if cfg.FailFast == nil {
		return false
	}
	return *cfg.FailFast
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
	_ = conn.Close()
	return true, nil
}
