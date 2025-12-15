# 🚀 Jaeger + ManticoreSearch 快速启动指南

## ✅ 系统已部署成功！

### 📍 访问地址

- **Jaeger UI**: http://192.168.5.15:30686
- **Collector OTLP gRPC**: `192.168.5.15:30317` (NodePort)
- **Collector OTLP HTTP**: `192.168.5.15:30318` (NodePort)

### 🧪 运行测试（从宿主机）

#### 方法 1: 使用 kubectl port-forward（推荐）

```bash
# 终端 1: 启动端口转发（保持运行）
kubectl port-forward -n tracing svc/jaeger-collector 4317:4317

# 终端 2: 运行测试
cd /Users/tal/dock/goutils
go clean -testcache
go test -v ./jaegerv1 -run TestGet1
```

#### 方法 2: 直接使用 NodePort

修改 `jaegerv1/comm_jaeger.go` 第 63 行：
```go
otlptracegrpc.WithEndpoint("192.168.5.15:30317"),
```

### 📊 验证数据存储

```bash
# 进入 VM
limactl shell k3s-vm

# 查询 ManticoreSearch 中的 span 数量
kubectl exec -n tracing deployment/manticore -- sh -c \
  "curl -s 'http://localhost:9308/sql' -d 'mode=raw&query=SELECT COUNT(*) FROM jaeger_spans'"

# 查看最近的 spans
kubectl exec -n tracing deployment/manticore -- sh -c \
  "curl -s 'http://localhost:9308/sql' -d 'mode=raw&query=SELECT trace_id, service_name, operation_name FROM jaeger_spans LIMIT 10'"
```

### 🔧 插件重新编译和部署

```bash
# 在 Lima VM 中执行
cd /Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin

export PATH=/usr/local/go/bin:$PATH

# 编译（ARM64 架构）
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
  -a -installsuffix cgo -ldflags '-w -s' \
  -o jaeger-mysql-plugin .

# 重启插件 Pod
kubectl delete pod -n tracing -l app=jaeger-mysql-plugin

# 等待新 Pod 启动
kubectl get pods -n tracing -l app=jaeger-mysql-plugin
```

### 📝 查看日志

```bash
# Jaeger Collector 日志
kubectl logs -n tracing -l component=collector --tail=50

# MySQL Plugin 日志
kubectl logs -n tracing -l app=jaeger-mysql-plugin --tail=50

# ManticoreSearch 日志
kubectl logs -n tracing -l app=manticore --tail=50

# Jaeger Query 日志
kubectl logs -n tracing -l component=query --tail=50
```

### 🗂️ 目录结构

```
lianlu/
├── k3s/
│   ├── 01-namespace.yaml              # 命名空间
│   ├── 02-manticore.yaml              # ManticoreSearch 部署
│   ├── 03-jaeger-clean.yaml           # Jaeger 基础组件
│   └── 04-jaeger-mysql-storage.yaml   # MySQL 插件集成
├── jaeger-mysql-plugin/
│   ├── main.go                        # 插件入口
│   ├── store.go                       # 存储实现
│   ├── go.mod
│   └── deploy-hostpath.sh             # 部署脚本
└── simple/
    ├── main_test.go                   # Go 测试示例
    └── README.md
```

### 🔍 故障排查

#### Collector CrashLoopBackOff
```bash
kubectl logs -n tracing -l component=collector --tail=100
```

#### Plugin 无法写入数据
```bash
# 检查插件日志中的错误
kubectl logs -n tracing -l app=jaeger-mysql-plugin | grep -i error

# 验证 ManticoreSearch 连接
kubectl exec -n tracing deployment/manticore -- \
  sh -c "curl -s 'http://localhost:9308/sql' -d 'mode=raw&query=SHOW TABLES'"
```

#### Go 测试没有发送 traces
```bash
# 确保端口转发正在运行
ps aux | grep "port-forward"

# 或检查 NodePort 连接
nc -zv 192.168.5.15 30317
```

### 🎯 关键配置说明

#### ManticoreSearch 与 MySQL 兼容性

ManticoreSearch 通过 MySQL 协议（端口 9306）兼容，但有限制：
- ❌ 不支持服务端预处理语句 (prepared statements)
- ✅ 解决方案：在 DSN 中添加 `interpolateParams=true`
- ✅ 使用 RT (Real-Time) 表进行插入和查询

#### 插件 DSN 配置

```go
dsn := "root:@tcp(manticore:9306)/?parseTime=true&multiStatements=true&interpolateParams=true"
```

`interpolateParams=true` 是关键！它让 Go MySQL driver 在客户端进行参数插值，避免使用 ManticoreSearch 不支持的服务端预处理语句。

### 📚 参考文档

- [完整部署文档](./COMPLETE_DEPLOYMENT.md)
- [插件开发文档](./jaeger-mysql-plugin/README.md)
- [测试指南](./jaegerv1/TESTING_GUIDE.md)

---

**🎊 恭喜！您的 Jaeger + ManticoreSearch 分布式追踪系统已完全就绪！**



