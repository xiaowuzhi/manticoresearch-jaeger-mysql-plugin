# Jaeger MySQL 存储插件

自定义 Jaeger 存储插件，使用 ManticoreSearch 的 MySQL 协议作为存储后端。

## 🎯 目标

解决 ManticoreSearch 的 Elasticsearch API 兼容性问题，通过 MySQL 协议（端口 9306）实现 Jaeger 数据存储。

## 🏗️ 架构

```
┌─────────────┐
│   Go App    │
└──────┬──────┘
       │ OTLP/gRPC
       ▼
┌─────────────────────────────┐
│   Jaeger Collector          │
│   (gRPC Plugin Mode)        │
└──────┬──────────────────────┘
       │ gRPC (17271)
       ▼
┌─────────────────────────────┐
│   MySQL Storage Plugin      │
│   (自定义 Go 应用)          │
└──────┬──────────────────────┘
       │ MySQL Protocol (9306)
       ▼
┌─────────────────────────────┐
│     ManticoreSearch         │
│   (MySQL 兼容接口)          │
└─────────────────────────────┘
       ▲
       │ MySQL Protocol (9306)
┌──────┴──────────────────────┐
│     Jaeger Query            │
│   (gRPC Plugin Mode)        │
└─────────────────────────────┘
```

## 📁 项目结构

```
jaeger-mysql-plugin/
├── main.go                  # 主程序入口
├── store.go                 # MySQL 存储实现
├── go.mod                   # Go 依赖
├── Dockerfile               # Docker 构建文件
├── build-and-deploy.sh      # 一键构建部署脚本
└── README.md                # 本文件
```

## 🚀 快速开始

### 前提条件

1. **K3s 环境**（已安装，使用 containerd 运行时）
2. **Docker**（用于构建镜像）
3. **Go 1.21+**（用于编译）
4. **ManticoreSearch 已部署**

### 环境说明

✅ **已适配 containerd/crictl 环境**

本项目的部署脚本已经完全适配 K3s 的 containerd 运行时：
- 使用 `ctr --namespace k8s.io` 导入镜像
- 使用 `crictl` 验证镜像
- `imagePullPolicy: Never` 用于本地镜像

详细说明请查看: [CONTAINERD.md](./CONTAINERD.md)

### 一键部署

#### 方式 1: 在 Lima K3s VM 中（无 Docker）⭐ 推荐

```bash
cd /Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin

./build-without-docker.sh
```

**特点**：
- ✅ 不需要 Docker
- ✅ 直接在 K3s 节点构建
- ✅ 自动检测构建工具（nerdctl/buildah/静态构建）
- ✅ 更轻量

**该脚本会**：
1. 检查环境（Go, kubectl）
2. 使用 Go 编译静态二进制
3. 创建容器镜像（或使用 ConfigMap）
4. 部署到 K3s

详见：[NO_DOCKER.txt](./NO_DOCKER.txt)

#### 方式 2: 在宿主机（需要 Docker）

```bash
./build-and-deploy.sh
```

**特点**：
- ❌ 需要 Docker
- ✅ 在宿主机构建
- ✅ 通过 Lima 导入到 VM

**该脚本会**：
1. 检查环境（Docker, kubectl）
2. 构建 Docker 镜像
3. 导入镜像到 K3s containerd
4. 部署到 K3s

### 手动部署

#### 1. 构建镜像

```bash
cd jaeger-mysql-plugin

# 初始化依赖
go mod tidy

# 构建 Docker 镜像
docker build -t jaeger-mysql-plugin:latest .
```

#### 2. 导入到 K3s (containerd)

```bash
# 保存镜像
docker save jaeger-mysql-plugin:latest -o /tmp/jaeger-mysql-plugin.tar

# 导入到 K3s containerd（通过 Lima）
limactl copy /tmp/jaeger-mysql-plugin.tar k3s-vm:/tmp/
limactl shell k3s-vm sudo ctr --namespace k8s.io images import /tmp/jaeger-mysql-plugin.tar

# 验证导入
limactl shell k3s-vm sudo crictl images | grep jaeger-mysql-plugin

# 清理
limactl shell k3s-vm rm /tmp/jaeger-mysql-plugin.tar
rm /tmp/jaeger-mysql-plugin.tar
```

**注意**: 必须使用 `--namespace k8s.io`，这是 K3s 的 containerd namespace。

#### 3. 部署到 Kubernetes

```bash
# 部署 ManticoreSearch（如果未部署）
kubectl apply -f ../k3s/02-manticore.yaml

# 部署 Jaeger + MySQL 插件
kubectl apply -f ../k3s/04-jaeger-mysql-storage.yaml
```

## 🔍 验证部署

### 查看 Pods 状态

```bash
kubectl get pods -n tracing
```

应该看到：
```
NAME                                   READY   STATUS    RESTARTS   AGE
manticore-xxx                          1/1     Running   0          5m
jaeger-mysql-plugin-xxx                1/1     Running   0          2m
jaeger-collector-grpc-xxx              1/1     Running   0          2m
jaeger-query-grpc-xxx                  1/1     Running   0          2m
```

### 查看日志

```bash
# MySQL 插件日志
kubectl logs -n tracing -l app=jaeger-mysql-plugin -f

# Collector 日志
kubectl logs -n tracing -l component=collector-grpc -f

# Query 日志
kubectl logs -n tracing -l component=query-grpc -f
```

### 测试连接

```bash
# 测试 ManticoreSearch MySQL 端口
kubectl exec -n tracing deployment/manticore -- nc -zv localhost 9306

# 测试插件 gRPC 端口
kubectl exec -n tracing deployment/jaeger-mysql-plugin -- nc -zv localhost 17271
```

## 🌐 访问 Jaeger UI

```
http://localhost:30686
```

## 📊 数据库结构

MySQL 插件会在 ManticoreSearch 中创建以下表：

### jaeger_spans 表

```sql
CREATE TABLE jaeger_spans (
    trace_id VARCHAR(32) NOT NULL,
    span_id VARCHAR(16) NOT NULL,
    operation_name TEXT NOT NULL,
    flags INT NOT NULL,
    start_time BIGINT NOT NULL,
    duration BIGINT NOT NULL,
    tags TEXT,
    logs TEXT,
    refs TEXT,
    process TEXT,
    service_name VARCHAR(255) NOT NULL,
    INDEX(trace_id),
    INDEX(service_name),
    INDEX(start_time)
);
```

### 字段说明

- `trace_id`: 追踪 ID
- `span_id`: Span ID
- `operation_name`: 操作名称
- `flags`: Span 标志
- `start_time`: 开始时间（纳秒）
- `duration`: 持续时间（纳秒）
- `tags`: 标签（JSON）
- `logs`: 日志（JSON）
- `refs`: 引用（JSON）
- `process`: 进程信息（JSON）
- `service_name`: 服务名称

## 🔧 配置选项

### MySQL 插件配置

```bash
--grpc-addr=:17271           # gRPC 监听地址
--mysql-addr=manticore:9306  # MySQL 地址
--mysql-db=jaeger            # 数据库名称
--mysql-user=root            # MySQL 用户名
--mysql-pass=                # MySQL 密码
```

### Jaeger Collector 配置

```yaml
env:
- name: SPAN_STORAGE_TYPE
  value: grpc-plugin
- name: GRPC_STORAGE_PLUGIN_SERVER
  value: jaeger-mysql-plugin:17271
- name: GRPC_STORAGE_PLUGIN_TLS
  value: "false"
```

## 🐛 故障排查

### 问题 1: MySQL 插件无法启动

```bash
# 查看详细日志
kubectl logs -n tracing -l app=jaeger-mysql-plugin

# 常见原因：
# 1. ManticoreSearch 未运行
# 2. MySQL 端口不可访问
# 3. 数据库初始化失败
```

### 问题 2: Collector 无法连接插件

```bash
# 查看 Collector 日志
kubectl logs -n tracing -l component=collector-grpc

# 检查插件服务
kubectl get svc jaeger-mysql-plugin -n tracing

# 测试连接
kubectl exec -n tracing deployment/jaeger-collector-grpc \
  -- nc -zv jaeger-mysql-plugin 17271
```

### 问题 3: 数据未保存

```bash
# 检查 ManticoreSearch 连接
kubectl exec -n tracing deployment/manticore \
  -- mysql -h127.0.0.1 -P9306 -e "SHOW TABLES"

# 检查是否有数据
kubectl exec -n tracing deployment/manticore \
  -- mysql -h127.0.0.1 -P9306 jaeger -e "SELECT COUNT(*) FROM jaeger_spans"
```

### 问题 4: Query 无法查询数据

```bash
# 确认 Query 使用相同的插件
kubectl get deployment jaeger-query-grpc -n tracing \
  -o jsonpath='{.spec.template.spec.containers[0].env[?(@.name=="GRPC_STORAGE_PLUGIN_SERVER")].value}'

# 应该输出: jaeger-mysql-plugin:17271
```

## 📝 技术细节

### 为什么不直接使用 Elasticsearch API？

ManticoreSearch 的 Elasticsearch API 兼容性有限：
- JSON 响应格式不完全兼容（布尔值 vs 字符串）
- 索引模板功能不完整
- Jaeger 创建索引时会失败

### 为什么使用 MySQL 协议？

- ManticoreSearch 的 MySQL 协议兼容性更好
- 支持标准的 SQL 操作
- 更稳定可靠

### gRPC Storage Plugin 机制

Jaeger 支持通过 gRPC 插件扩展存储后端：
1. 插件实现 `StoragePlugin` gRPC 接口
2. Jaeger Collector 通过 gRPC 调用插件存储数据
3. Jaeger Query 通过 gRPC 调用插件查询数据

## 🔗 相关资源

- [Jaeger gRPC Storage Plugin](https://github.com/jaegertracing/jaeger/tree/main/plugin/storage/grpc)
- [ManticoreSearch MySQL 协议](https://manual.manticoresearch.com/Connecting_to_ManticoreSearch/MySQL_protocol)
- [Jaeger Storage API](https://www.jaegertracing.io/docs/latest/deployment/#storage-plugins)

## ⚠️ 注意事项

1. **ManticoreSearch 限制**：
   - 不是完整的关系型数据库
   - DDL 支持有限
   - 某些 SQL 特性可能不支持

2. **性能考虑**：
   - 大量数据写入时性能可能受限
   - 建议定期清理旧数据

3. **生产环境**：
   - 这是实验性解决方案
   - 生产环境推荐使用真实的 Elasticsearch 或 Cassandra

## 📈 性能优化

### 批量写入

修改 `store.go` 中的 `WriteSpan` 方法，实现批量插入：

```go
// 使用批量插入提高性能
// INSERT INTO jaeger_spans VALUES (...), (...), (...)
```

### 索引优化

根据查询模式调整索引：

```sql
-- 添加复合索引
CREATE INDEX idx_service_time ON jaeger_spans(service_name, start_time);
CREATE INDEX idx_trace ON jaeger_spans(trace_id, start_time);
```

### 数据清理

定期清理旧数据：

```bash
# 删除 7 天前的数据
kubectl exec -n tracing deployment/manticore -- \
  mysql -h127.0.0.1 -P9306 jaeger -e \
  "DELETE FROM jaeger_spans WHERE start_time < UNIX_TIMESTAMP(DATE_SUB(NOW(), INTERVAL 7 DAY)) * 1000000000"
```

## 🎯 下一步

1. **测试追踪**：运行 `../simple/` 目录中的 Go 应用
2. **查看数据**：在 Jaeger UI 中搜索服务
3. **监控性能**：观察插件和 ManticoreSearch 的资源使用

## 🛠️ 开发

### 修改代码

```bash
# 编辑 main.go 或 store.go

# 重新构建和部署
./build-and-deploy.sh
```

### 本地测试

```bash
# 编译
go build -o jaeger-mysql-plugin

# 运行（需要 ManticoreSearch 可访问）
./jaeger-mysql-plugin \
  --grpc-addr=:17271 \
  --mysql-addr=localhost:9306 \
  --mysql-db=jaeger
```

---

**快速开始**: `./build-and-deploy.sh` 🚀

