# Jaeger + ManticoreSearch 分布式追踪系统

**完整的生产级 Jaeger 部署方案，使用 ManticoreSearch 作为 MySQL 兼容存储后端**

## 🎯 项目概述

本项目实现了在 K3s (Lima VM, ARM64) 上部署 Jaeger 分布式追踪系统，使用自定义的 MySQL 存储插件连接 ManticoreSearch。

### 核心特性

- ✅ 完整的 Jaeger 组件（Collector, Query, Agent）
- ✅ 自定义 Go gRPC 存储插件（ARM64）
- ✅ ManticoreSearch 作为 MySQL 兼容存储
- ✅ 支持 OTLP (gRPC/HTTP) 协议
- ✅ 统一的命名规范和网络配置

## 📁 目录结构

```
lianlu/
├── README.md                          # 本文档 ⭐
├── COMPLETE_DEPLOYMENT.md             # 完整部署文档（最重要）⭐
├──  k3s/                               # Kubernetes 配置
│   ├── 01-namespace.yaml              # 命名空间
│   ├── 02-manticore.yaml              # ManticoreSearch 部署
│   ├── 03-jaeger-clean.yaml           # Jaeger (Elasticsearch) 参考配置
│   ├── 04-jaeger-mysql-storage.yaml   # Jaeger + MySQL Plugin 完整配置 ⭐
│   ├── deploy-manticore-only.sh       # ManticoreSearch 单独部署脚本
│   ├── jaeger-deploy.sh               # Jaeger 部署脚本
│   ├── README.md                      # K3s 配置说明
│   └── MYSQL_STORAGE_SOLUTION.md      # MySQL 存储方案文档
├── jaeger-mysql-plugin/               # 自定义存储插件
│   ├── main.go                        # 插件主程序 ⭐
│   ├── store.go                       # 存储接口实现 ⭐
│   ├── go.mod                         # Go 依赖
│   ├── go.sum
│   ├── jaeger-mysql-plugin            # 编译的 ARM64 二进制 ⭐
│   ├── Dockerfile                     # Docker 构建文件
│   ├── deploy-hostpath.sh             # 部署脚本（hostPath 方式）⭐
│   ├── INSTALL_GO_IN_VM.sh            # Go 安装和构建脚本
│   ├── README.md                      # 插件文档
│   ├── QUICKSTART.txt                 # 快速开始
│   ├── HOW_TO_RUN.txt                 # 运行指南
│   ├── NO_DOCKER.txt                  # 无 Docker 构建说明
│   └── CONTAINERD.md                  # Containerd 环境说明
└── simple/                            # Go 测试示例
    ├── main.go                        # 主程序
    ├── main_test.go                   # 测试用例
    ├── otel_tracer.go                 # OpenTelemetry tracer
    ├── otel_tracer_test.go            # Tracer 测试
    ├── README.md                      # 测试说明
    └── OTEL_TEST_README.md            # OTEL 测试文档
```

## 🚀 快速开始

### 1. 部署系统

```bash
# 在 Lima K3s VM 中
cd /Users/tal/dock/goutils/k3s/lianlu

# 方式 A: 使用一键脚本
cd jaeger-mysql-plugin
./deploy-hostpath.sh

# 方式 B: 手动部署
kubectl apply -f k3s/01-namespace.yaml
kubectl apply -f k3s/02-manticore.yaml
# 等待 ManticoreSearch 启动
sleep 30
kubectl apply -f k3s/04-jaeger-mysql-storage.yaml
```

### 2. 验证部署

```bash
# 查看所有组件
kubectl get pods -n tracing

# 应该看到所有 Pods 都是 Running 状态：
# - manticore
# - jaeger-mysql-plugin
# - jaeger-collector
# - jaeger-query
# - jaeger-agent
```

### 3. 发送测试 Trace

```bash
# 使用 simple 目录中的测试代码
cd simple
./run.sh
```

## 📚 文档

### 主要文档

1. **[COMPLETE_DEPLOYMENT.md](./COMPLETE_DEPLOYMENT.md)** ⭐
   - 完整的部署指南
   - 架构说明
   - 使用方法
   - 故障排查

2. **[k3s/README.md](./k3s/README.md)**
   - K3s 配置说明
   - YAML 文件详解

3. **[jaeger-mysql-plugin/README.md](./jaeger-mysql-plugin/README.md)**
   - 插件技术文档
   - 开发指南

4. **[simple/README.md](./simple/README.md)**
   - Go 测试示例
   - OTLP 使用方法

### 快速参考

- **[jaeger-mysql-plugin/QUICKSTART.txt](./jaeger-mysql-plugin/QUICKSTART.txt)** - 快速开始
- **[jaeger-mysql-plugin/HOW_TO_RUN.txt](./jaeger-mysql-plugin/HOW_TO_RUN.txt)** - 运行指南
- **[k3s/MYSQL_STORAGE_SOLUTION.md](./k3s/MYSQL_STORAGE_SOLUTION.md)** - MySQL 存储方案

## 🏗️ 架构

```
应用 → Agent → Collector → MySQL Plugin → ManticoreSearch
                               ↓
                           Query ← Web UI
```

### 组件说明

| 组件 | 用途 | 端口 |
|------|------|------|
| **Collector** | 接收 traces | 4317(OTLP), 14250(Jaeger) |
| **Query** | Web UI 和 API | 16686, NodePort 30686 |
| **Agent** | 本地代理 | 6831, 6832 |
| **MySQL Plugin** | 存储插件 | 17271 |
| **ManticoreSearch** | 数据存储 | 9306(MySQL), 9308(HTTP) |

## 🔧 常用命令

### 查看状态

```bash
# 所有资源
kubectl get all -n tracing

# Pod 日志
kubectl logs -n tracing -l app=jaeger,component=collector -f
kubectl logs -n tracing -l app=jaeger-mysql-plugin -f
```

### 查询数据

```bash
# 查询 ManticoreSearch
kubectl exec -it -n tracing deployment/manticore -- sh -c \
  "wget -q -O- 'http://localhost:9308/sql' --post-data='mode=raw&query=SELECT COUNT(*) FROM jaeger_spans'"

# Query API
kubectl exec -n tracing deployment/jaeger-query -- \
  wget -q -O- http://localhost:16686/api/services
```

### 重启组件

```bash
kubectl rollout restart deployment/jaeger-collector -n tracing
kubectl rollout restart deployment/jaeger-query -n tracing
kubectl rollout restart deployment/jaeger-mysql-plugin -n tracing
```

## 📝 开发

### 修改插件代码

```bash
cd jaeger-mysql-plugin

# 1. 修改代码 (main.go 或 store.go)

# 2. 重新编译（在 Lima VM 中）
export PATH=/usr/local/go/bin:$PATH
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 \
  go build -a -installsuffix cgo -ldflags '-w -s' -o jaeger-mysql-plugin .

# 3. 重启 Pod（会自动使用新二进制）
kubectl delete pod -n tracing -l app=jaeger-mysql-plugin

# 4. 查看日志
kubectl logs -n tracing -l app=jaeger-mysql-plugin -f
```

### 运行测试

```bash
cd simple
go test -v ./...
```

## 🎯 下一步

1. **集成应用** - 在您的微服务中添加 Jaeger 客户端
2. **发送数据** - 配置应用发送 traces 到 Collector
3. **查看 UI** - 访问 Jaeger UI 查看追踪数据
4. **优化配置** - 根据负载调整资源和采样率

## 📖 相关资源

- [Jaeger 官方文档](https://www.jaegertracing.io/docs/)
- [OpenTelemetry 文档](https://opentelemetry.io/docs/)
- [ManticoreSearch 文档](https://manual.manticoresearch.com/)

## ✨ 技术亮点

- ✅ 自定义 gRPC 存储插件
- ✅ MySQL 协议兼容性
- ✅ ARM64 原生支持
- ✅ 无 Docker 构建流程
- ✅ hostPath 部署策略
- ✅ 完整的测试用例

---

**🎉 完整的生产级 Jaeger 分布式追踪系统！**

详细文档请查看 [COMPLETE_DEPLOYMENT.md](./COMPLETE_DEPLOYMENT.md)
# manticoresearch-jaeger-mysql-plugin
