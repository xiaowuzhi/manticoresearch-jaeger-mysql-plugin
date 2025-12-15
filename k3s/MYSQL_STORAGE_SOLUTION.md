# Jaeger MySQL 存储解决方案

## 🎯 问题回顾

**原始问题**: ManticoreSearch 的 Elasticsearch API 与 Jaeger 不完全兼容

```
{"level":"fatal","msg":"Failed to create span writer",
 "error":"json: cannot unmarshal string into Go struct field 
  IndicesPutTemplateResponse.acknowledged of type bool"}
```

## ✅ 解决方案

**自定义 MySQL 存储插件**：通过 ManticoreSearch 的 MySQL 协议（端口 9306）实现 Jaeger 数据存储。

## 🏗️ 技术架构

```
┌──────────────────────────────────────────────────────────────┐
│                     应用层                                    │
│                                                              │
│  Go App → Jaeger Agent → Jaeger Collector (gRPC Plugin)    │
└──────────────────────┬───────────────────────────────────────┘
                       │
                       │ gRPC (17271)
                       ▼
┌──────────────────────────────────────────────────────────────┐
│                  自定义存储层                                 │
│                                                              │
│         jaeger-mysql-plugin (Go gRPC Server)                │
│         • SpanWriter: 写入 spans                            │
│         • SpanReader: 查询 spans                            │
│         • MySQL Client: 连接 ManticoreSearch                │
└──────────────────────┬───────────────────────────────────────┘
                       │
                       │ MySQL Protocol (9306)
                       ▼
┌──────────────────────────────────────────────────────────────┐
│                  存储层                                       │
│                                                              │
│            ManticoreSearch                                   │
│            • MySQL 兼容接口                                  │
│            • 数据持久化 (PVC)                                │
│            • 表: jaeger_spans                                │
└──────────────────────┬───────────────────────────────────────┘
                       ▲
                       │ gRPC (17271)
┌──────────────────────┴───────────────────────────────────────┐
│                  查询层                                       │
│                                                              │
│      Jaeger Query (gRPC Plugin) → jaeger-mysql-plugin       │
│                      ↓                                       │
│               Jaeger UI (30686)                              │
└──────────────────────────────────────────────────────────────┘
```

## 📁 项目结构

```
lianlu/
├── jaeger-mysql-plugin/          # 自定义存储插件
│   ├── main.go                   # 主程序（gRPC Server）
│   ├── store.go                  # 存储实现（MySQL 操作）
│   ├── go.mod                    # Go 依赖
│   ├── Dockerfile                # Docker 构建
│   ├── build-and-deploy.sh       # 一键部署脚本
│   ├── README.md                 # 详细文档
│   └── QUICKSTART.txt            # 快速开始
│
└── k3s/
    ├── 02-manticore.yaml         # ManticoreSearch 部署
    └── 04-jaeger-mysql-storage.yaml  # Jaeger + MySQL 插件部署
```

## 🚀 快速部署

### 一键部署（推荐）

```bash
cd /Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin
./build-and-deploy.sh
```

### 部署流程

1. **构建插件镜像**
   ```bash
   docker build -t jaeger-mysql-plugin:latest .
   ```

2. **导入到 K3s**
   ```bash
   docker save jaeger-mysql-plugin:latest -o /tmp/jaeger-mysql-plugin.tar
   limactl copy /tmp/jaeger-mysql-plugin.tar k3s-vm:/tmp/
   limactl shell k3s-vm sudo ctr --namespace k8s.io images import /tmp/jaeger-mysql-plugin.tar
   ```

3. **部署 Kubernetes 资源**
   ```bash
   kubectl apply -f ../k3s/02-manticore.yaml
   kubectl apply -f ../k3s/04-jaeger-mysql-storage.yaml
   ```

## 📊 组件说明

### 1. ManticoreSearch

**作用**: 提供 MySQL 兼容的存储接口

**配置**:
- MySQL 端口: 9306
- HTTP 端口: 9308（未使用）
- 存储: PVC (10Gi)

**数据库结构**:
```sql
CREATE TABLE jaeger_spans (
    trace_id VARCHAR(32) NOT NULL,
    span_id VARCHAR(16) NOT NULL,
    operation_name TEXT NOT NULL,
    flags INT NOT NULL,
    start_time BIGINT NOT NULL,
    duration BIGINT NOT NULL,
    tags TEXT,           -- JSON
    logs TEXT,           -- JSON
    refs TEXT,           -- JSON
    process TEXT,        -- JSON
    service_name VARCHAR(255) NOT NULL,
    INDEX(trace_id),
    INDEX(service_name),
    INDEX(start_time)
);
```

### 2. MySQL Storage Plugin

**作用**: Jaeger 和 ManticoreSearch 之间的桥梁

**功能**:
- gRPC Server (端口 17271)
- 实现 Jaeger StoragePlugin 接口
- SpanWriter: 将 spans 写入 MySQL
- SpanReader: 从 MySQL 查询 spans
- 自动创建数据库和表

**环境变量**:
```yaml
--grpc-addr=:17271
--mysql-addr=manticore:9306
--mysql-db=jaeger
--mysql-user=root
--mysql-pass=
```

### 3. Jaeger Collector (gRPC Mode)

**作用**: 接收追踪数据，通过 gRPC 插件存储

**配置**:
```yaml
SPAN_STORAGE_TYPE: grpc-plugin
GRPC_STORAGE_PLUGIN_SERVER: jaeger-mysql-plugin:17271
GRPC_STORAGE_PLUGIN_TLS: "false"
COLLECTOR_OTLP_ENABLED: "true"
```

**端口**:
- 14250: Jaeger gRPC
- 14268: Jaeger HTTP
- 4317: OTLP gRPC
- 4318: OTLP HTTP
- 9411: Zipkin

### 4. Jaeger Query (gRPC Mode)

**作用**: 查询追踪数据，提供 UI

**配置**:
```yaml
SPAN_STORAGE_TYPE: grpc-plugin
GRPC_STORAGE_PLUGIN_SERVER: jaeger-mysql-plugin:17271
```

**访问**: http://localhost:30686

## 🔍 验证和测试

### 1. 检查部署状态

```bash
kubectl get pods -n tracing

# 应该看到：
# manticore-xxx              1/1  Running
# jaeger-mysql-plugin-xxx    1/1  Running
# jaeger-collector-grpc-xxx  1/1  Running
# jaeger-query-grpc-xxx      1/1  Running
```

### 2. 查看日志

```bash
# MySQL 插件
kubectl logs -n tracing -l app=jaeger-mysql-plugin -f

# Collector
kubectl logs -n tracing -l component=collector-grpc -f

# 应该看到：
# {"level":"info","msg":"Successfully connected to MySQL"}
# {"level":"info","msg":"Starting gRPC server","address":":17271"}
```

### 3. 测试连接

```bash
# ManticoreSearch MySQL 端口
kubectl exec -n tracing deployment/manticore -- nc -zv localhost 9306

# 插件 gRPC 端口
kubectl exec -n tracing deployment/jaeger-mysql-plugin -- nc -zv localhost 17271
```

### 4. 验证数据

```bash
# 查看表
kubectl exec -n tracing deployment/manticore -- \
  mysql -h127.0.0.1 -P9306 -e "SHOW TABLES FROM jaeger"

# 查看数据
kubectl exec -n tracing deployment/manticore -- \
  mysql -h127.0.0.1 -P9306 jaeger -e "SELECT COUNT(*) FROM jaeger_spans"
```

## 📈 性能和限制

### 性能特点

**优点**:
- ✅ MySQL 协议稳定
- ✅ 数据持久化
- ✅ 支持索引查询
- ✅ 避免 ES API 兼容性问题

**限制**:
- ⚠️ ManticoreSearch 不是完整的关系型数据库
- ⚠️ 某些 SQL 特性可能不支持
- ⚠️ 大规模数据写入性能待测试

### 优化建议

1. **批量写入**: 修改插件实现批量 INSERT
2. **索引优化**: 根据查询模式调整索引
3. **数据清理**: 定期删除旧数据
4. **资源调整**: 根据负载调整 CPU/内存

## 🔄 vs 其他方案对比

| 方案 | 稳定性 | 持久化 | 兼容性 | 适用场景 |
|------|--------|--------|--------|----------|
| **MySQL Plugin** | ⭐⭐⭐⭐ | ✅ | ✅ | 开发/测试/小规模生产 |
| Memory | ⭐⭐⭐⭐⭐ | ❌ | ✅ | 开发/测试 |
| ManticoreSearch ES API | ⭐⭐ | ✅ | ❌ | 不推荐 |
| Elasticsearch | ⭐⭐⭐⭐⭐ | ✅ | ✅ | 生产环境 |
| Cassandra | ⭐⭐⭐⭐ | ✅ | ✅ | 大规模生产 |

## 🐛 故障排查

### 问题 1: 插件无法启动

**症状**: `jaeger-mysql-plugin` Pod CrashLoopBackOff

**排查**:
```bash
kubectl logs -n tracing -l app=jaeger-mysql-plugin
```

**常见原因**:
- ManticoreSearch 未运行或不可达
- MySQL 端口 9306 不可访问
- 数据库初始化失败

### 问题 2: Collector 无法连接插件

**症状**: Collector 日志显示 "failed to connect to plugin"

**排查**:
```bash
kubectl logs -n tracing -l component=collector-grpc
kubectl get svc jaeger-mysql-plugin -n tracing
```

**解决**:
- 确认插件 Service 存在
- 确认插件 Pod 正在运行
- 测试 gRPC 端口连通性

### 问题 3: 数据未保存

**症状**: Jaeger UI 中看不到追踪数据

**排查**:
```bash
# 检查 Collector 是否接收到数据
kubectl logs -n tracing -l component=collector-grpc | grep -i span

# 检查数据库
kubectl exec -n tracing deployment/manticore -- \
  mysql -h127.0.0.1 -P9306 jaeger -e "SELECT COUNT(*) FROM jaeger_spans"
```

### 问题 4: Query 查询失败

**症状**: UI 无法显示追踪数据

**排查**:
```bash
kubectl logs -n tracing -l component=query-grpc
```

**确认**: Query 使用相同的插件配置

## 🔐 安全考虑

1. **MySQL 密码**: 当前未设置密码，生产环境应使用 Secret
2. **gRPC TLS**: 当前未启用 TLS，生产环境应启用
3. **网络策略**: 考虑使用 NetworkPolicy 限制访问
4. **RBAC**: 限制 ServiceAccount 权限

## 📚 参考资源

- [Jaeger gRPC Storage Plugin](https://github.com/jaegertracing/jaeger/tree/main/plugin/storage/grpc)
- [ManticoreSearch MySQL Protocol](https://manual.manticoresearch.com/Connecting_to_ManticoreSearch/MySQL_protocol)
- [Go MySQL Driver](https://github.com/go-sql-driver/mysql)

## 🎯 下一步

1. **部署系统**: 运行 `./build-and-deploy.sh`
2. **测试追踪**: 运行 `../simple/` 中的测试应用
3. **查看数据**: 访问 Jaeger UI
4. **监控性能**: 观察资源使用和查询响应时间

---

**立即开始**: `cd jaeger-mysql-plugin && ./build-and-deploy.sh` 🚀



