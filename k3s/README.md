# Jaeger + ManticoreSearch 分布式追踪系统

K3s 环境下的 Jaeger 分布式追踪系统，使用 ManticoreSearch 作为存储后端。

## 🚀 快速开始

### 一键部署

```bash
./jaeger.sh
```

选择 `4) 快速部署` 即可自动部署所有组件。

### 或手动部署

```bash
# 1. 创建命名空间
kubectl apply -f 01-namespace.yaml

# 2. 部署 ManticoreSearch
kubectl apply -f 02-manticore.yaml

# 3. 部署 Jaeger
kubectl apply -f 03-jaeger-clean.yaml
```

## 🪁 部署 Kite 面板（zxh326/kite）

Kite 项目地址：[`zxh326/kite`](https://github.com/zxh326/kite)

> 说明：本仓库提供的 `06-kite.yaml` 用的是 **cluster-admin**（方便先跑起来）。生产环境请务必收紧权限。

### 安装

```bash
kubectl apply -f 06-kite.yaml
```

### 访问方式

- **NodePort（默认）**：`http://<任意节点IP>:30081`
- **Port-forward（无需暴露端口）**

```bash
kubectl -n kube-system port-forward svc/kite 8080:8080
```

访问：`http://127.0.0.1:8080`

## 🧭 部署 Kubernetes Dashboard（Web 面板）

> 说明：这是 **Kubernetes Dashboard**（和上面的 Kite 不同，二选一/按需安装）。
> 这里提供一个**开发/演示**用的 Dashboard 部署（带 `admin-user` 的 `cluster-admin` 权限）。生产环境请务必收紧 RBAC。

### 安装

```bash
kubectl apply -f 05-kubernetes-dashboard.yaml
```

### 获取登录 Token

```bash
kubectl -n kubernetes-dashboard create token admin-user
```

### 访问方式

- **NodePort（默认）**：`https://<任意节点IP>:30080`
  - 浏览器会提示自签名证书不受信任，选择继续访问即可
- **Port-forward（无需暴露端口）**

```bash
kubectl -n kubernetes-dashboard port-forward svc/kubernetes-dashboard 8443:443
```

访问：`https://127.0.0.1:8443`

## 📋 管理工具

### 主工具

```bash
./jaeger.sh          # 主管理工具（推荐）
```

提供统一的管理界面，包括：
- 部署管理
- 诊断工具
- 存储管理
- 快捷操作

### 独立工具

```bash
./jaeger-deploy.sh    # 部署管理（部署/重部署/清理）
./jaeger-diagnose.sh  # 诊断工具（状态/日志/连接测试）
./jaeger-storage.sh   # 存储管理（切换 ManticoreSearch/Memory）
```

## 🔍 访问 Jaeger UI

部署完成后访问：

```
http://localhost:30686
```

## 📊 查看状态

```bash
# 快速查看
./jaeger.sh  # 选择 5) 快速查看状态

# 或使用 kubectl
kubectl get all -n tracing
```

## 🛠️ 常见操作

### 查看日志

```bash
# 使用诊断工具
./jaeger-diagnose.sh  # 选择相应选项

# 或直接查看
kubectl logs -n tracing -l component=collector --tail=50
kubectl logs -n tracing -l component=query --tail=50
kubectl logs -n tracing -l app=manticore --tail=50
```

### 切换存储后端

```bash
./jaeger-storage.sh
```

支持：
- **ManticoreSearch**（生产推荐，数据持久化）
- **Memory**（开发/测试，数据不持久化）

### 重新部署

```bash
./jaeger-deploy.sh  # 选择 2) 重新部署
```

### 清理资源

```bash
./jaeger.sh  # 选择 6) 快速清理
# 或
./jaeger-deploy.sh  # 选择 4) 仅清理
```

## 🏗️ 系统架构

```
┌─────────────┐
│   Go App    │ ──────┐
└─────────────┘       │
                      ▼
┌─────────────────────────────┐
│     Jaeger Agent (DS)       │
└─────────────────────────────┘
              │
              ▼
┌─────────────────────────────┐
│   Jaeger Collector          │
│   Port: 14250 (gRPC)        │
│         4317 (OTLP)         │
└─────────────────────────────┘
              │
              ▼ (Elasticsearch API)
┌─────────────────────────────┐
│     ManticoreSearch         │
│   Port: 9308 (HTTP)         │
│         9306 (MySQL)        │
└─────────────────────────────┘
              ▲
              │
┌─────────────────────────────┐
│     Jaeger Query UI         │
│   Port: 16686 -> 30686      │
└─────────────────────────────┘
```

## 📁 文件说明

### YAML 配置

- `01-namespace.yaml` - 命名空间
- `02-manticore.yaml` - ManticoreSearch 部署
- `03-jaeger-clean.yaml` - Jaeger 组件部署

### 管理脚本

- `jaeger.sh` - 主管理工具（统一入口）
- `jaeger-deploy.sh` - 部署管理
- `jaeger-diagnose.sh` - 诊断工具
- `jaeger-storage.sh` - 存储管理
- `test-deployment.sh` - 部署测试

### 文档

- `README.md` - 本文件
- `QUICKSTART.md` - 详细快速开始指南
- `ARCHITECTURE.md` - 架构说明
- `STORAGE_OPTIONS.md` - 存储选项说明

## 🐛 故障排查

### 查看 Pod 状态

```bash
kubectl get pods -n tracing
```

### 检查 Collector 日志

```bash
kubectl logs -n tracing -l component=collector --tail=50
```

### 运行诊断

```bash
./jaeger-diagnose.sh  # 选择 6) 完整诊断报告
```

### 常见问题

**Q: Collector 连接 ManticoreSearch 失败？**

A: 切换到内存存储
```bash
./jaeger-storage.sh  # 选择 3) 切换到内存存储
```

**Q: ManticoreSearch 配置文件只读？**

A: 已修复，使用 initContainer 复制配置到可写目录

**Q: Pod 一直处于 Pending 状态？**

A: 检查存储类是否可用
```bash
kubectl get storageclass
kubectl describe pvc -n tracing
```

## 🔗 相关链接

- [Jaeger 官方文档](https://www.jaegertracing.io/)
- [ManticoreSearch 文档](https://manual.manticoresearch.com/)
- [K3s 文档](https://docs.k3s.io/)

## 📝 注意事项

1. **存储后端**：默认使用 ManticoreSearch，但连接可能不稳定，建议开发环境使用内存存储
2. **数据持久化**：ManticoreSearch 使用 PVC，数据会持久化；Memory 模式数据不持久化
3. **性能**：ManticoreSearch 的 Elasticsearch API 兼容性有限，生产环境建议使用真实的 Elasticsearch
4. **版本**：Jaeger 使用最新版本，请注意 v1 将在 2025-12-31 EOL，建议未来迁移到 v2

## 🎯 测试 Go 应用

查看 `../simple/` 目录中的 Go 应用示例：

```bash
cd ../simple
go test -v
./run.sh
```

## 📊 验证追踪

1. 运行测试应用（在 `../simple/` 目录）
2. 访问 Jaeger UI: http://localhost:30686
3. 搜索服务名称查看追踪数据

## 🔄 更新系统

```bash
# 更新 Jaeger
kubectl apply -f 03-jaeger-clean.yaml
kubectl rollout restart deployment -n tracing

# 更新 ManticoreSearch
kubectl apply -f 02-manticore.yaml
kubectl rollout restart deployment/manticore -n tracing
```

---

**开始使用**: 运行 `./jaeger.sh` 即可！
