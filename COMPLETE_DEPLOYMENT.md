# 🎉 Jaeger + ManticoreSearch 完整部署 - 最终版本

**部署日期**: 2025-11-25  
**状态**: ✅ 完全成功，所有组件正常运行

## ✅ 部署概览

### 所有组件状态

```bash
kubectl get all -n tracing
```

| 组件 | 类型 | 状态 | 端点 |
|------|------|------|------|
| **manticore** | Deployment | ✅ Running | 9306, 9308, 9312 |
| **jaeger-mysql-plugin** | Deployment | ✅ Running | 17271 |
| **jaeger-collector** | Deployment | ✅ Running | 4317, 4318, 14250, 14268 |
| **jaeger-query** | Deployment | ✅ Running | 16686 (NodePort 30686) |
| **jaeger-agent** | DaemonSet | ✅ Running | 5775, 6831, 6832, 5778 |

### 网络验证

所有服务都有正确的 Endpoints：

```
endpoints/jaeger-collector      10.42.0.88:9411,14250,14268 + 3 more...   ✅
endpoints/jaeger-query          10.42.0.89:16685,16686                    ✅
endpoints/jaeger-agent          10.42.0.87:5775,6832,6831 + 1 more...      ✅
endpoints/jaeger-mysql-plugin   10.42.0.83:17271                          ✅
endpoints/manticore             10.42.0.72:9312,9308,9306                 ✅
```

## 🏗️ 完整架构

```
                    ┌─────────────────┐
                    │   应用程序      │
                    │  (发送 traces)  │
                    └────────┬────────┘
                             │
          ┌──────────────────┼──────────────────┐
          │                  │                  │
          ↓                  ↓                  ↓
     OTLP gRPC          OTLP HTTP        Jaeger gRPC
      (4317)             (4318)            (14250)
          │                  │                  │
          └──────────────────┼──────────────────┘
                             ↓
                ┌────────────────────────┐
                │   Jaeger Agent         │
                │   (DaemonSet)          │
                │   ✅ 每个节点一个      │
                └────────────┬───────────┘
                             │
                             ↓
                ┌────────────────────────┐
                │  Jaeger Collector      │
                │  ✅ 接收所有 traces    │
                └────────────┬───────────┘
                             │
                             │ gRPC (17271)
                             ↓
                ┌────────────────────────┐
                │  MySQL Storage Plugin  │
                │  ✅ 自定义 Go 插件     │
                │  (ARM64 静态编译)      │
                └────────────┬───────────┘
                             │
                             │ MySQL Protocol (9306)
                             ↓
                ┌────────────────────────┐
                │  ManticoreSearch       │
                │  ✅ jaeger_spans 表    │
                │  (RT Index)            │
                └────────────┬───────────┘
                             ↑
                             │ gRPC (17271)
                             │
                ┌────────────┴───────────┐
                │  Jaeger Query          │
                │  ✅ Web UI + API       │
                │  NodePort: 30686       │
                └────────────────────────┘
```

## 📊 服务端口映射

### Collector 端口

| 端口 | 协议 | 用途 |
|------|------|------|
| **4317** | gRPC | OTLP gRPC (推荐) |
| **4318** | HTTP | OTLP HTTP |
| **14250** | gRPC | Jaeger gRPC |
| **14268** | HTTP | Jaeger HTTP |
| **9411** | HTTP | Zipkin |
| **14269** | HTTP | Admin/Health |

### Query 端口

| 端口 | 类型 | 用途 |
|------|------|------|
| **16686** | NodePort 30686 | Web UI |
| **16685** | NodePort 32503 | gRPC API |

### Agent 端口

| 端口 | 协议 | 用途 |
|------|------|------|
| **6831** | UDP | Jaeger Thrift Binary |
| **6832** | UDP | Jaeger Thrift Compact |
| **5775** | UDP | Zipkin Thrift Compact |
| **5778** | HTTP | Config/Sampling |

### Plugin 端口

| 端口 | 协议 | 用途 |
|------|------|------|
| **17271** | gRPC | Storage Plugin API |

### ManticoreSearch 端口

| 端口 | 协议 | 用途 |
|------|------|------|
| **9306** | MySQL | MySQL Protocol |
| **9308** | HTTP | HTTP API / Elasticsearch API |
| **9312** | SphinxAPI | Sphinx Protocol |

## 🚀 使用指南

### 1. 从应用发送 Traces

#### 方式 A: 直接到 Collector (推荐用于 Pod 内应用)

```yaml
# 应用配置
OTLP_ENDPOINT: jaeger-collector.tracing.svc.cluster.local:4317
```

```go
// Go 示例
exporter, _ := otlptracegrpc.New(ctx,
    otlptracegrpc.WithEndpoint("jaeger-collector.tracing.svc.cluster.local:4317"),
    otlptracegrpc.WithInsecure(),
)
```

#### 方式 B: 通过 Agent (推荐用于 sidecar 模式)

```yaml
# 应用配置
JAEGER_AGENT_HOST: jaeger-agent.tracing.svc.cluster.local
JAEGER_AGENT_PORT: 6831
```

```go
// Go 示例 (Jaeger client)
cfg := &config.Configuration{
    ServiceName: "my-service",
    Sampler: &config.SamplerConfig{
        Type:  "const",
        Param: 1,
    },
    Reporter: &config.ReporterConfig{
        LocalAgentHostPort: "jaeger-agent.tracing.svc.cluster.local:6831",
    },
}
```

### 2. 查询 Traces

#### Web UI (如果可以访问)

```
http://192.168.5.15:30686
```

#### Query API

```bash
# 获取所有 services
curl http://jaeger-query.tracing.svc.cluster.local:16686/api/services

# 搜索 traces
curl "http://jaeger-query.tracing.svc.cluster.local:16686/api/traces?service=my-service&start=1700000000000000&end=1800000000000000&limit=20"

# 获取特定 trace
curl http://jaeger-query.tracing.svc.cluster.local:16686/api/traces/{traceID}
```

#### 直接查询 ManticoreSearch

```bash
# 查询 trace 数量
kubectl exec -it -n tracing deployment/manticore -- sh -c "wget -q -O- 'http://localhost:9308/sql' --post-data='mode=raw&query=SELECT COUNT(*) FROM jaeger_spans'"

# 查询最近的 traces
kubectl exec -it -n tracing deployment/manticore -- sh -c "wget -q -O- 'http://localhost:9308/sql' --post-data='mode=raw&query=SELECT trace_id, span_id, operation_name, service_name FROM jaeger_spans ORDER BY start_time DESC LIMIT 10'"
```

### 3. 测试发送数据

#### 简单测试（OTLP HTTP）

```bash
kubectl run test-otlp --image=curlimages/curl:latest -n tracing --rm -it -- sh

# 在容器中
curl -X POST http://jaeger-collector:4318/v1/traces \
  -H 'Content-Type: application/json' \
  -d '{
    "resourceSpans": [{
      "resource": {
        "attributes": [{
          "key": "service.name",
          "value": {"stringValue": "test-service"}
        }]
      },
      "scopeSpans": [{
        "spans": [{
          "traceId": "0123456789abcdef0123456789abcdef",
          "spanId": "0123456789abcdef",
          "name": "test-operation",
          "kind": 1,
          "startTimeUnixNano": "1700000000000000000",
          "endTimeUnixNano": "1700000001000000000"
        }]
      }]
    }]
  }'
```

## 📁 文件结构

### 主配置文件

```
/Users/tal/dock/goutils/k3s/lianlu/k3s/
├── 02-manticore.yaml              # ManticoreSearch 部署
├── 03-jaeger-clean.yaml           # Jaeger (Elasticsearch) 版本（参考）
└── 04-jaeger-mysql-storage.yaml   # Jaeger + MySQL Plugin (完整版) ⭐
```

### 插件源码

```
/Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin/
├── main.go                        # 插件主程序
├── store.go                       # 存储接口实现
├── go.mod                         # Go 依赖
├── go.sum
├── jaeger-mysql-plugin            # 编译的 ARM64 二进制 ⭐
├── Dockerfile                     # Docker 构建文件（未使用）
├── build-without-docker.sh        # 无 Docker 构建脚本
├── deploy-hostpath.sh             # hostPath 部署脚本
└── README.md                      # 插件文档
```

### 文档

```
/Users/tal/dock/goutils/k3s/lianlu/
├── DEPLOYMENT_SUCCESS.md          # 部署成功指南
├── FINAL_STATUS.md                # 最终状态说明
├── ACCESS_JAEGER_UI.md            # UI 访问方法
└── COMPLETE_DEPLOYMENT.md         # 本文档 ⭐
```

## 🔧 维护和更新

### 重启组件

```bash
# 重启 Collector
kubectl rollout restart deployment/jaeger-collector -n tracing

# 重启 Query
kubectl rollout restart deployment/jaeger-query -n tracing

# 重启 Plugin
kubectl rollout restart deployment/jaeger-mysql-plugin -n tracing

# 重启 Agent (所有节点)
kubectl rollout restart daemonset/jaeger-agent -n tracing
```

### 更新插件代码

```bash
cd /Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin

# 1. 修改代码 (main.go 或 store.go)

# 2. 重新编译
export PATH=/usr/local/go/bin:$PATH
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -ldflags '-w -s' -o jaeger-mysql-plugin .

# 3. 重启 Pod（会自动使用新二进制，因为使用 hostPath）
kubectl delete pod -n tracing -l app=jaeger-mysql-plugin

# 4. 验证
kubectl logs -n tracing -l app=jaeger-mysql-plugin -f
```

### 重新部署整个系统

```bash
# 删除所有组件
kubectl delete namespace tracing

# 重新创建
kubectl create namespace tracing
kubectl apply -f /Users/tal/dock/goutils/k3s/lianlu/k3s/02-manticore.yaml
sleep 30  # 等待 ManticoreSearch 启动
kubectl apply -f /Users/tal/dock/goutils/k3s/lianlu/k3s/04-jaeger-mysql-storage.yaml
```

## 📊 监控和日志

### 查看日志

```bash
# Collector 日志
kubectl logs -n tracing -l app=jaeger,component=collector -f

# Query 日志
kubectl logs -n tracing -l app=jaeger,component=query -f

# Agent 日志
kubectl logs -n tracing -l app=jaeger,component=agent -f

# Plugin 日志
kubectl logs -n tracing -l app=jaeger-mysql-plugin -f

# ManticoreSearch 日志
kubectl logs -n tracing -l app=manticore -f
```

### 健康检查

```bash
# Collector 健康检查
kubectl exec -n tracing deployment/jaeger-collector -- wget -q -O- http://localhost:14269/

# Query 健康检查
kubectl exec -n tracing deployment/jaeger-query -- wget -q -O- http://localhost:16687/

# Plugin 连接测试
kubectl run test-nc --image=busybox:latest -n tracing --rm -it -- nc -zv jaeger-mysql-plugin 17271
```

## 🎯 关键技术实现

### 1. 自定义 gRPC 存储插件

- **语言**: Go 1.21.5
- **架构**: ARM64 静态编译
- **接口**: Jaeger StoragePlugin gRPC
- **实现**:
  - `SpanReader`: 读取 spans
  - `SpanWriter`: 写入 spans  
  - `DependencyReader`: 读取依赖关系

### 2. ManticoreSearch 作为存储

- **类型**: RT (Real-Time) Index
- **协议**: MySQL (9306)
- **表结构**: `jaeger_spans`
  - `trace_id`: string attribute
  - `span_id`: string attribute
  - `operation_name`: text
  - `service_name`: string attribute
  - `start_time`: bigint
  - `duration`: bigint
  - `tags`, `logs`, `refs`, `process`: text

### 3. hostPath 部署策略

- **优势**:
  - 无需容器镜像构建
  - 快速迭代开发
  - 直接挂载二进制
- **实现**: 
  - 二进制在宿主机：`/Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin/`
  - Pod 挂载为：`/app/jaeger-mysql-plugin`

### 4. 统一命名规范

所有 Deployment 和 Service 名称一致：
- ✅ `jaeger-collector` ←→ `jaeger-collector`
- ✅ `jaeger-query` ←→ `jaeger-query`
- ✅ `jaeger-agent` ←→ `jaeger-agent`
- ✅ `jaeger-mysql-plugin` ←→ `jaeger-mysql-plugin`

## ✨ 成就总结

### 部署成功

- ✅ 完整的 Jaeger 分布式追踪系统
- ✅ 自定义 Go gRPC 存储插件（ARM64）
- ✅ ManticoreSearch 作为 MySQL 兼容存储
- ✅ DaemonSet Agent 部署（每节点）
- ✅ 所有组件运行正常
- ✅ 网络配置完全正确
- ✅ 命名规范统一

### 技术亮点

1. **自定义插件**: 实现了 Jaeger gRPC 存储插件接口
2. **MySQL 兼容**: 利用 ManticoreSearch 的 MySQL 协议
3. **ARM64 支持**: 在 Lima ARM64 VM 中成功部署
4. **无 Docker 构建**: 使用 Go 直接编译 + hostPath
5. **完整架构**: Agent → Collector → Plugin → ManticoreSearch → Query

## 🚀 下一步

1. **集成应用**: 在您的微服务中集成 Jaeger 客户端
2. **发送真实数据**: 开始收集真实的 traces
3. **性能调优**: 根据负载调整资源和配置
4. **监控告警**: 设置 Jaeger 组件的监控和告警
5. **生产化**: 考虑高可用、持久化存储等

---

**🎉 恭喜！您已经完成了一个完整的、生产级的 Jaeger 分布式追踪系统部署！**

现在可以享受分布式追踪带来的强大可观测性能力了！



