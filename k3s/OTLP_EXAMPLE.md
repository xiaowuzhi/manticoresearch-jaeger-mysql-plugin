# OTLP (OpenTelemetry Protocol) 使用指南

## 🎯 为什么使用 OTLP？

OTLP 是 OpenTelemetry 的标准协议，推荐使用：

- ✅ **现代化**: OpenTelemetry 是 CNCF 孵化项目
- ✅ **统一**: 支持 Traces、Metrics、Logs
- ✅ **高效**: gRPC 协议，性能优秀
- ✅ **标准**: 跨语言、跨平台统一标准

## 📡 Collector 支持的 OTLP 端口

- **4317**: OTLP gRPC（推荐，高性能）
- **4318**: OTLP HTTP（兼容性好）

## 🔧 Go 应用使用 OTLP

### 方式 1: 使用 OpenTelemetry SDK

#### 安装依赖

```bash
go get go.opentelemetry.io/otel
go get go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc
go get go.opentelemetry.io/otel/sdk/trace
go get go.opentelemetry.io/otel/sdk/resource
go get go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp
```

#### 代码示例

```go
package main

import (
    "context"
    "log"
    "time"

    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    "go.opentelemetry.io/otel/sdk/resource"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
    semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
    "go.opentelemetry.io/otel/trace"
    "google.golang.org/grpc"
    "google.golang.org/grpc/credentials/insecure"
)

// InitOTLP 初始化 OTLP Tracer
func InitOTLP(serviceName, collectorEndpoint string) (func(), error) {
    ctx := context.Background()

    // 创建 OTLP gRPC Exporter
    exporter, err := otlptracegrpc.New(ctx,
        otlptracegrpc.WithEndpoint(collectorEndpoint),
        otlptracegrpc.WithInsecure(), // 生产环境应使用 TLS
        otlptracegrpc.WithDialOption(grpc.WithBlock()),
    )
    if err != nil {
        return nil, err
    }

    // 创建资源
    res, err := resource.New(ctx,
        resource.WithAttributes(
            semconv.ServiceName(serviceName),
            semconv.ServiceVersion("1.0.0"),
        ),
    )
    if err != nil {
        return nil, err
    }

    // 创建 TracerProvider
    tp := sdktrace.NewTracerProvider(
        sdktrace.WithBatcher(exporter),
        sdktrace.WithResource(res),
        sdktrace.WithSampler(sdktrace.AlwaysSample()),
    )

    // 设置全局 TracerProvider
    otel.SetTracerProvider(tp)

    // 返回清理函数
    return func() {
        ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
        defer cancel()
        if err := tp.Shutdown(ctx); err != nil {
            log.Printf("Error shutting down tracer provider: %v", err)
        }
    }, nil
}

// 使用示例
func main() {
    // 初始化 OTLP
    // K8s 内使用: jaeger-collector.tracing.svc.cluster.local:4317
    // 本地测试使用: localhost:4317
    cleanup, err := InitOTLP("my-service", "jaeger-collector.tracing.svc.cluster.local:4317")
    if err != nil {
        log.Fatal(err)
    }
    defer cleanup()

    // 获取 Tracer
    tracer := otel.Tracer("my-service")

    // 创建 Span
    ctx := context.Background()
    ctx, span := tracer.Start(ctx, "main-operation")
    defer span.End()

    // 添加属性
    span.SetAttributes(
        attribute.String("user.id", "12345"),
        attribute.Int("http.status_code", 200),
    )

    // 添加事件
    span.AddEvent("Processing started")

    // 执行业务逻辑
    doSomething(ctx, tracer)

    span.AddEvent("Processing completed")

    log.Println("Trace sent to Jaeger via OTLP!")
}

func doSomething(ctx context.Context, tracer trace.Tracer) {
    // 创建子 Span
    _, span := tracer.Start(ctx, "sub-operation")
    defer span.End()

    // 模拟工作
    time.Sleep(100 * time.Millisecond)

    span.SetAttributes(attribute.String("result", "success"))
}
```

### 方式 2: 使用 OTLP HTTP (端口 4318)

```go
import (
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracehttp"
)

// 创建 HTTP Exporter
exporter, err := otlptracehttp.New(ctx,
    otlptracehttp.WithEndpoint("jaeger-collector.tracing.svc.cluster.local:4318"),
    otlptracehttp.WithInsecure(),
)
```

## 🐳 K8s 部署配置

### Deployment 示例

```yaml
apiVersion: apps/v1
kind: Deployment
metadata:
  name: otlp-app
  namespace: tracing
spec:
  replicas: 1
  selector:
    matchLabels:
      app: otlp-app
  template:
    metadata:
      labels:
        app: otlp-app
    spec:
      containers:
      - name: app
        image: your-app:latest
        env:
        # OTLP Endpoint
        - name: OTEL_EXPORTER_OTLP_ENDPOINT
          value: "jaeger-collector.tracing.svc.cluster.local:4317"
        # Service Name
        - name: OTEL_SERVICE_NAME
          value: "my-service"
        # 采样率 (1.0 = 100%)
        - name: OTEL_TRACES_SAMPLER
          value: "always_on"
        # Protocol (grpc 或 http/protobuf)
        - name: OTEL_EXPORTER_OTLP_PROTOCOL
          value: "grpc"
```

## 🧪 测试 OTLP 连接

### 使用 telemetrygen 工具

```bash
# 在 K8s 中运行测试
kubectl run otlp-test --image=otel/telemetrygen:latest \
  -n tracing --rm -it -- \
  traces \
  --otlp-endpoint jaeger-collector:4317 \
  --otlp-insecure \
  --duration 30s \
  --rate 10

# 在 Jaeger UI 中查看生成的追踪
open http://localhost:30686
```

### 使用 curl 测试 OTLP HTTP

```bash
kubectl run curl-test --image=curlimages/curl:latest \
  --rm -it -n tracing -- sh

# 测试 OTLP HTTP 端口
curl -X POST http://jaeger-collector:4318/v1/traces \
  -H "Content-Type: application/json" \
  -d '{}'
```

## 🔄 从 Jaeger Client 迁移到 OTLP

### 旧代码 (Jaeger Client)

```go
import (
    "github.com/uber/jaeger-client-go"
    "github.com/uber/jaeger-client-go/config"
)

cfg := &config.Configuration{
    ServiceName: "my-service",
    Sampler: &config.SamplerConfig{
        Type:  "const",
        Param: 1,
    },
    Reporter: &config.ReporterConfig{
        LocalAgentHostPort: "jaeger-agent:6831",
    },
}
tracer, closer, _ := cfg.NewTracer()
```

### 新代码 (OpenTelemetry + OTLP)

```go
import (
    "go.opentelemetry.io/otel"
    "go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
    sdktrace "go.opentelemetry.io/otel/sdk/trace"
)

exporter, _ := otlptracegrpc.New(context.Background(),
    otlptracegrpc.WithEndpoint("jaeger-collector:4317"),
    otlptracegrpc.WithInsecure(),
)

tp := sdktrace.NewTracerProvider(
    sdktrace.WithBatcher(exporter),
    sdktrace.WithSampler(sdktrace.AlwaysSample()),
)
otel.SetTracerProvider(tp)

tracer := otel.Tracer("my-service")
```

## 📊 性能对比

| 协议 | 端口 | 性能 | 兼容性 | 推荐 |
|------|------|------|--------|------|
| Jaeger UDP | 6831 | 最快 | Jaeger only | 遗留项目 |
| Jaeger gRPC | 14250 | 快 | Jaeger only | 遗留项目 |
| OTLP gRPC | 4317 | 快 | 标准协议 | ⭐ 推荐 |
| OTLP HTTP | 4318 | 中等 | 标准协议 | 兼容性 |

## 🌐 多语言支持

### Python

```python
from opentelemetry import trace
from opentelemetry.exporter.otlp.proto.grpc.trace_exporter import OTLPSpanExporter
from opentelemetry.sdk.trace import TracerProvider
from opentelemetry.sdk.trace.export import BatchSpanProcessor

# 配置 OTLP Exporter
otlp_exporter = OTLPSpanExporter(
    endpoint="jaeger-collector.tracing.svc.cluster.local:4317",
    insecure=True
)

# 设置 TracerProvider
provider = TracerProvider()
processor = BatchSpanProcessor(otlp_exporter)
provider.add_span_processor(processor)
trace.set_tracer_provider(provider)

# 使用
tracer = trace.get_tracer(__name__)
with tracer.start_as_current_span("operation"):
    print("Hello, OTLP!")
```

### Java

```java
import io.opentelemetry.api.OpenTelemetry;
import io.opentelemetry.exporter.otlp.trace.OtlpGrpcSpanExporter;
import io.opentelemetry.sdk.OpenTelemetrySdk;
import io.opentelemetry.sdk.trace.SdkTracerProvider;
import io.opentelemetry.sdk.trace.export.BatchSpanProcessor;

OtlpGrpcSpanExporter spanExporter = OtlpGrpcSpanExporter.builder()
    .setEndpoint("http://jaeger-collector.tracing.svc.cluster.local:4317")
    .build();

SdkTracerProvider tracerProvider = SdkTracerProvider.builder()
    .addSpanProcessor(BatchSpanProcessor.builder(spanExporter).build())
    .build();

OpenTelemetry openTelemetry = OpenTelemetrySdk.builder()
    .setTracerProvider(tracerProvider)
    .build();
```

### Node.js

```javascript
const { NodeTracerProvider } = require('@opentelemetry/sdk-trace-node');
const { OTLPTraceExporter } = require('@opentelemetry/exporter-trace-otlp-grpc');
const { BatchSpanProcessor } = require('@opentelemetry/sdk-trace-base');

const exporter = new OTLPTraceExporter({
  url: 'grpc://jaeger-collector.tracing.svc.cluster.local:4317',
});

const provider = new NodeTracerProvider();
provider.addSpanProcessor(new BatchSpanProcessor(exporter));
provider.register();

// 使用
const tracer = provider.getTracer('my-service');
const span = tracer.startSpan('operation');
// ... do work ...
span.end();
```

## 🔍 验证 OTLP 配置

### 检查 Collector 配置

```bash
# 查看 Collector 环境变量
kubectl exec -n tracing deployment/jaeger-collector -- env | grep OTLP

# 应该看到:
# COLLECTOR_OTLP_ENABLED=true
```

### 检查端口是否开放

```bash
# 端口转发测试
kubectl port-forward -n tracing svc/jaeger-collector 4317:4317

# 在另一个终端测试
grpcurl -plaintext localhost:4317 list
```

### 查看 Collector 日志

```bash
kubectl logs -n tracing -l component=collector --tail=100 -f

# 应该看到接收到的 OTLP 数据日志
```

## 📚 参考资源

- [OpenTelemetry 官方文档](https://opentelemetry.io/docs/)
- [OTLP 规范](https://github.com/open-telemetry/opentelemetry-proto)
- [Jaeger OTLP 支持](https://www.jaegertracing.io/docs/features/)
- [Go OpenTelemetry SDK](https://github.com/open-telemetry/opentelemetry-go)

## 💡 最佳实践

1. **使用 gRPC (4317)**：性能更好
2. **批处理**：使用 BatchSpanProcessor 而不是 SimpleSpanProcessor
3. **采样**：根据流量调整采样率
4. **超时配置**：设置合理的超时时间
5. **错误处理**：优雅处理 Exporter 错误
6. **资源属性**：添加 service.name、service.version 等
7. **上下文传播**：使用标准的 W3C Trace Context

## ⚠️ 常见问题

### Q: OTLP gRPC 连接失败？

```bash
# 检查 Service
kubectl get svc -n tracing jaeger-collector

# 检查端口
kubectl get svc -n tracing jaeger-collector -o yaml | grep 4317
```

### Q: 数据没有显示在 Jaeger UI？

1. 检查 Collector 日志是否有错误
2. 确认 OTLP 已启用
3. 检查存储配置
4. 验证采样率设置

### Q: 性能问题？

1. 使用 BatchSpanProcessor
2. 调整批处理大小和超时
3. 考虑使用异步 Exporter
4. 增加 Collector 副本数

## 🎓 学习路径

1. **基础**: 理解 OpenTelemetry 概念
2. **实践**: 运行本文档的示例代码
3. **集成**: 在现有应用中集成 OTLP
4. **优化**: 调整性能和采样配置
5. **监控**: 添加 Metrics 和 Logs

---

**开始使用 OTLP！** 🚀

