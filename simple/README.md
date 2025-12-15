# 🎯 超级简单的 Jaeger 追踪示例

**无 Web 框架，无部署，纯粹的追踪测试！**

## ✨ 特点

- ✅ 不使用 Gin 或任何 Web 框架
- ✅ 纯粹的 Go 函数 + Jaeger 追踪
- ✅ 完整的测试用例
- ✅ 3 个简单示例（HelloWorld, Add, ProcessOrder）
- ✅ 可以独立运行或仅测试

## 🚀 快速开始

### 方式 1: 仅运行测试（不需要 Jaeger）

```bash
cd simple

# 运行测试
go test -v

# 带覆盖率
go test -v -cover

# Benchmark
go test -bench=.
```

### 方式 2: 运行程序并发送追踪数据到 Jaeger

**前提**: Jaeger 必须在运行（本地 6831 端口）

```bash
# 1. 启动 Jaeger（如果还没启动）
docker run -d --name jaeger \
  -p 6831:6831/udp \
  -p 16686:16686 \
  jaegertracing/all-in-one:latest

# 2. 运行程序
cd simple
go run main.go

# 3. 访问 Jaeger UI 查看追踪
open http://localhost:16686
```

## 📝 代码结构

### main.go (约150行)

包含 3 个示例：

**示例 1: HelloWorld**
```go
HelloWorld(ctx, "张三")  // 返回: "Hello, 张三!"
```

**示例 2: Add**
```go
Add(ctx, 10, 20)  // 返回: 30
```

**示例 3: ProcessOrder（多层调用）**
```go
ProcessOrder(ctx, "ORDER-001")
  ├─ validateOrder()      // 验证订单
  ├─ calculateAmount()    // 计算金额
  └─ saveOrder()          // 保存订单
```

### main_test.go (约200行)

包含 11 个测试用例：

1. `TestHelloWorld` - 基本功能测试
2. `TestHelloWorldWithDifferentNames` - 多种输入测试
3. `TestAdd` - 加法测试
4. `TestProcessOrder` - 订单处理测试
5. `TestValidateOrder` - 订单验证测试
6. `TestCalculateAmount` - 金额计算测试
7. `TestSaveOrder` - 保存订单测试
8. `TestInitJaeger` - Jaeger 初始化测试
9. `TestSpanTagsAndLogs` - Span 标签和日志测试
10. `BenchmarkHelloWorld` - 性能测试
11. `BenchmarkAdd` - 性能测试
12. `BenchmarkProcessOrder` - 性能测试

## 🧪 测试

### 运行所有测试

```bash
go test -v
```

输出示例：
```
=== RUN   TestHelloWorld
--- PASS: TestHelloWorld (0.00s)
=== RUN   TestHelloWorldWithDifferentNames
--- PASS: TestHelloWorldWithDifferentNames (0.00s)
=== RUN   TestAdd
--- PASS: TestAdd (0.00s)
=== RUN   TestProcessOrder
--- PASS: TestProcessOrder (0.09s)
...
PASS
coverage: 92.3% of statements
ok      simple-jaeger-demo      0.234s
```

### 覆盖率测试

```bash
go test -cover -coverprofile=coverage.out
go tool cover -html=coverage.out -o coverage.html
open coverage.html
```

### Benchmark 测试

```bash
go test -bench=. -benchmem
```

输出示例：
```
BenchmarkHelloWorld-8       50000    24567 ns/op    1234 B/op    12 allocs/op
BenchmarkAdd-8             100000    12345 ns/op     678 B/op     8 allocs/op
BenchmarkProcessOrder-8     10000   101234 ns/op    2345 B/op    23 allocs/op
```

## 📊 运行示例

### 运行程序

```bash
go run main.go
```

输出：
```
2024/11/21 10:00:00 Jaeger Tracer 初始化成功: simple-demo

=== 示例 1: Hello World ===
Hello, 张三!

=== 示例 2: 简单计算 ===
10 + 20 = 30

=== 示例 3: 处理订单（多层调用）===
Order ORDER-001 processed successfully

等待数据上报...

✓ 完成！请访问 Jaeger UI 查看追踪数据
  http://localhost:16686
```

### 在 Jaeger UI 中查看

1. 访问 http://localhost:16686
2. 选择服务: `simple-demo`
3. 点击 "Find Traces"
4. 查看详细的追踪信息

## 🎯 不需要

❌ **不需要以下任何东西来运行测试**：
- Web 服务器
- Gin 框架
- K8s 集群
- Docker（测试时）
- Jaeger 服务（测试时）

✅ **只需要**：
- Go 1.21+
- 源代码

## 💡 关键函数

### InitJaeger - 初始化追踪器

```go
tracer, closer := InitJaeger("my-service")
defer closer()
```

### 创建 Span

```go
span := tracer.StartSpan("operation-name")
defer span.Finish()

// 添加标签
span.SetTag("key", "value")

// 添加日志
span.LogKV("event", "something happened")
```

### 使用 Context 传递 Span

```go
span, ctx := opentracing.StartSpanFromContext(parentCtx, "operation")
defer span.Finish()

// 在子函数中使用
childFunction(ctx)
```

## 📚 依赖

```
github.com/opentracing/opentracing-go
github.com/uber/jaeger-client-go
github.com/stretchr/testify (仅测试)
```

## 🔧 配置

修改 Jaeger Agent 地址（main.go）：

```go
Reporter: &config.ReporterConfig{
    LocalAgentHostPort: "localhost:6831",  // 修改这里
}
```

修改采样率：

```go
Sampler: &config.SamplerConfig{
    Type:  "const",
    Param: 1,  // 1 = 100%, 0.1 = 10%
}
```

## 🎉 快速命令

```bash
# 仅测试（最简单）
go test -v

# 运行程序（需要 Jaeger）
go run main.go

# 完整测试
go test -v -cover -bench=.
```

## 📖 学习资源

- [OpenTracing 规范](https://opentracing.io/docs/)
- [Jaeger 文档](https://www.jaegertracing.io/docs/)
- [Jaeger Go Client](https://github.com/jaegertracing/jaeger-client-go)

---

**这是最简单的 Jaeger 追踪示例，非常适合学习和测试！** 🎊

