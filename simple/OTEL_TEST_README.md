# OpenTelemetry (OTLP) 测试指南

## 📖 说明

本目录包含两种 Jaeger 追踪实现：

1. **Jaeger Client** (legacy) - 使用 `github.com/uber/jaeger-client-go`
   - 文件: `main.go`, `main_test.go`
   - 协议: Jaeger UDP (6831)

2. **OpenTelemetry** (推荐) - 使用 `go.opentelemetry.io/otel` ⭐
   - 文件: `otel_tracer.go`, `otel_tracer_test.go`
   - 协议: OTLP gRPC (4317)

## 🚀 快速开始

### 前置条件

确保 Jaeger 已在 K3s 中部署（启用 ManticoreSearch 存储）：

```bash
cd /Users/tal/dock/goutils/lianlu/k3s
./deploy.sh
```

### 运行 OTLP 测试

#### 1. 安装依赖

```bash
cd /Users/tal/dock/goutils/lianlu/simple
go mod tidy
```

#### 2. 本地测试（端口转发）

```bash
# 在一个终端中，转发 Collector OTLP 端口
kubectl port-forward -n tracing svc/jaeger-collector 4317:4317

# 在另一个终端中运行测试
cd /Users/tal/dock/goutils/lianlu/simple
go test -v -run TestOTEL
```

#### 3. K8s 内部测试

设置环境变量指向 K8s Service:

```bash
export OTEL_EXPORTER_OTLP_ENDPOINT="jaeger-collector.tracing.svc.cluster.local:4317"
go test -v -run TestOTEL
```

## 🧪 测试用例

### 基础测试

```bash
# 基础追踪
go test -v -run TestOTELBasic

# 数据库追踪
go test -v -run TestOTELDatabase

# HTTP 追踪
go test -v -run TestOTELHTTP
```

### 高级测试

```bash
# 嵌套 Span
go test -v -run TestOTELNestedSpans

# 错误追踪
go test -v -run TestOTELError

# 自定义属性
go test -v -run TestOTELWithCustomAttributes
```

### 运行所有 OTLP 测试

```bash
go test -v -run TestOTEL

# 或者
go test -v ./... -run OTEL
```

### 性能测试

```bash
go test -bench=BenchmarkOTELSpanCreation -benchmem
```

## 📊 查看追踪数据

### 1. 访问 Jaeger UI

```bash
# 通过 NodePort
open http://localhost:30686

# 或通过端口转发
kubectl port-forward -n tracing svc/jaeger-query 16686:16686
open http://localhost:16686
```

### 2. 在 UI 中查看

1. **Service**: 选择对应的服务名称
   - `test-service`
   - `database-service`
   - `http-service`
   - `nested-service`
   - `error-service`
   - 等等

2. **Operations**: 选择操作名称或留空

3. **Find Traces**: 点击搜索

4. **查看详情**: 点击任意追踪查看详细的 Span、Tag 和 Event

## 🔍 代码示例

### 基础使用

```go
import "go.opentelemetry.io/otel/trace"

// 1. 初始化 Tracer
config := OTELConfig{
    ServiceName: "my-service",
    Endpoint:    "jaeger-collector.tracing.svc.cluster.local:4317",
}
tracer, cleanup, err := NewOTELTrace(config)
if err != nil {
    log.Fatal(err)
}
defer cleanup()

// 2. 创建 Span
ctx := context.Background()
ctx, span := tracer.Start(ctx, "my-operation")
defer span.End()

// 3. 添加属性
span.SetAttributes(
    attribute.String("key", "value"),
    attribute.Int("count", 42),
)

// 4. 添加事件
span.AddEvent("Something happened")
```

### 嵌套 Span

```go
// 父 Span
ctx, parentSpan := tracer.Start(context.Background(), "parent")
defer parentSpan.End()

// 子 Span (使用父 Span 的 context)
_, childSpan := tracer.Start(ctx, "child")
defer childSpan.End()
```

### 错误处理

```go
ctx, span := tracer.Start(context.Background(), "operation")
defer span.End()

if err := doSomething(); err != nil {
    RecordError(span, err)
    return err
}
```

## 🎯 测试场景

### 测试 1: 基础追踪 (`TestOTELBasic`)

- 创建简单的 Span
- 添加属性和事件
- 验证 TraceID 和 SpanID

### 测试 2: 数据库追踪 (`TestOTELDatabase`)

- 模拟数据库查询
- 使用标准数据库语义属性
- 记录查询语句和结果

### 测试 3: HTTP 追踪 (`TestOTELHTTP`)

- 模拟 HTTP 请求
- 使用标准 HTTP 语义属性
- 记录状态码和响应信息

### 测试 4: 嵌套 Span (`TestOTELNestedSpans`)

- 创建父子 Span 关系
- 追踪多层调用链
- 验证 Context 传播

### 测试 5: 错误追踪 (`TestOTELError`)

- 记录错误信息
- 设置错误标记
- 添加错误事件

### 测试 6: 多操作 (`TestOTELMultipleOperations`)

- 连续创建多个 Span
- 测试批处理功能
- 验证数据上报

### 测试 7: 自定义属性 (`TestOTELWithCustomAttributes`)

- 添加业务相关属性
- 使用辅助函数简化操作
- 添加自定义事件

### 测试 8: 长时间运行 (`TestOTELLongRunning`)

- 模拟长时间操作
- 记录进度事件
- 追踪多步骤流程

## 🔧 配置说明

### OTELConfig 结构

```go
type OTELConfig struct {
    ServiceName string // 服务名称（必需）
    Endpoint    string // OTLP Collector 地址
    Token       string // 认证 Token (可选)
    HostName    string // 主机名
}
```

### 环境变量

| 变量 | 说明 | 默认值 |
|------|------|--------|
| `OTEL_EXPORTER_OTLP_ENDPOINT` | Collector 地址 | `localhost:4317` |
| `KUBERNETES_SERVICE_HOST` | K8s 环境检测 | - |
| `OTEL_BSP_MAX_QUEUE_SIZE` | 批处理队列大小 | `30` |
| `OTEL_BSP_MAX_EXPORT_BATCH_SIZE` | 批处理批次大小 | `10` |

### K8s vs 本地

代码会自动检测环境：

- **K8s 内部**: 使用 `jaeger-collector.tracing.svc.cluster.local:4317`
- **本地开发**: 使用 `localhost:4317` (需要端口转发)

## 📈 性能

### Benchmark 结果示例

```bash
$ go test -bench=BenchmarkOTELSpanCreation -benchmem

BenchmarkOTELSpanCreation-8   	   50000	     35000 ns/op	    2048 B/op	      25 allocs/op
```

### 优化建议

1. **使用批处理**: 默认已启用 BatchSpanProcessor
2. **调整队列大小**: 根据流量调整 `OTEL_BSP_MAX_QUEUE_SIZE`
3. **采样率**: 生产环境考虑降低采样率
4. **异步上报**: OTLP 默认异步，不阻塞主流程

## 🆚 对比 Jaeger Client

| 特性 | Jaeger Client | OpenTelemetry |
|------|---------------|---------------|
| 协议 | Jaeger UDP | OTLP gRPC ⭐ |
| 端口 | 6831 | 4317 |
| 标准 | Jaeger 专有 | CNCF 标准 |
| 语义属性 | 自定义 | 标准化 |
| 多后端 | 仅 Jaeger | 多种后端 |
| 推荐 | 遗留项目 | 新项目 |

## 🐛 故障排查

### 问题 1: 连接失败

```bash
# 检查 Collector 是否运行
kubectl get pods -n tracing -l component=collector

# 检查端口
kubectl get svc -n tracing jaeger-collector

# 测试连接
kubectl port-forward -n tracing svc/jaeger-collector 4317:4317
```

### 问题 2: 数据未显示

1. 确认 ManticoreSearch 已部署
2. 检查 Collector 日志: `kubectl logs -n tracing -l component=collector`
3. 确认存储配置正确
4. 等待数据写入（可能有延迟）

### 问题 3: 依赖问题

```bash
# 清理并重新安装
go clean -modcache
go mod tidy
go mod download
```

## 📚 参考文档

- [OpenTelemetry Go SDK](https://github.com/open-telemetry/opentelemetry-go)
- [OTLP 规范](https://github.com/open-telemetry/opentelemetry-proto)
- [Jaeger OTLP 支持](https://www.jaegertracing.io/docs/features/)
- [语义属性约定](https://opentelemetry.io/docs/specs/semconv/)

## 💡 最佳实践

1. **使用语义属性**: 使用 `semconv` 包中的标准属性
2. **Context 传播**: 始终传递 Context 以建立父子关系
3. **及时 End**: 使用 `defer span.End()` 确保 Span 结束
4. **错误记录**: 使用 `RecordError()` 记录错误
5. **事件而非日志**: 使用 AddEvent 而不是打印日志
6. **批处理**: 让 SDK 处理批处理，不要手动管理

## 🎓 下一步

1. **集成到应用**: 将追踪集成到实际应用中
2. **自动化**: 使用 Instrumentation 自动追踪
3. **监控**: 添加 Metrics 和 Logs
4. **优化**: 根据实际流量优化采样和批处理
5. **告警**: 基于追踪数据设置告警

---

**开始使用 OpenTelemetry！** 🚀

