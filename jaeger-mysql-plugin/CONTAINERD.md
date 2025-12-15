# containerd/crictl 环境说明

## 🎯 环境特点

您的 K3s 环境使用 **containerd** 作为容器运行时，而不是 Docker。

## 🔧 关键差异

### Docker vs containerd

| 操作 | Docker | containerd (K3s) |
|------|--------|------------------|
| 查看镜像 | `docker images` | `sudo crictl images` 或 `sudo ctr -n k8s.io images ls` |
| 导入镜像 | `docker load` | `sudo ctr -n k8s.io images import` |
| 查看容器 | `docker ps` | `sudo crictl ps` |
| 查看日志 | `docker logs` | `sudo crictl logs` |
| 删除镜像 | `docker rmi` | `sudo crictl rmi` 或 `sudo ctr -n k8s.io images rm` |

### imagePullPolicy

对于本地导入的镜像，必须使用：
```yaml
imagePullPolicy: Never  # 或 IfNotPresent
```

如果设置为 `Always`，K8s 会尝试从镜像仓库拉取，导致失败。

## 🚀 部署流程

### 1. 构建和部署（已适配 containerd）

```bash
cd /Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin

./build-and-deploy.sh
```

该脚本会：
1. 使用 Docker 构建镜像
2. 导出为 tar 文件
3. 使用 `ctr` 导入到 containerd（namespace: k8s.io）
4. 使用 `crictl` 验证镜像
5. 部署到 K8s

### 2. 验证镜像导入

```bash
./verify-containerd.sh
```

## 🔍 手动操作

### 导入镜像到 containerd

**在 Lima VM 中**:
```bash
# 1. 在本地构建并保存镜像
docker build -t jaeger-mysql-plugin:latest .
docker save jaeger-mysql-plugin:latest -o /tmp/jaeger-mysql-plugin.tar

# 2. 复制到 VM
limactl copy /tmp/jaeger-mysql-plugin.tar k3s-vm:/tmp/

# 3. 在 VM 中导入（注意 namespace）
limactl shell k3s-vm sudo ctr --namespace k8s.io images import /tmp/jaeger-mysql-plugin.tar

# 4. 验证
limactl shell k3s-vm sudo crictl images | grep jaeger-mysql-plugin

# 5. 清理
limactl shell k3s-vm rm /tmp/jaeger-mysql-plugin.tar
rm /tmp/jaeger-mysql-plugin.tar
```

**本地 K3s（非 Lima）**:
```bash
# 1. 构建并保存
docker build -t jaeger-mysql-plugin:latest .
docker save jaeger-mysql-plugin:latest -o /tmp/jaeger-mysql-plugin.tar

# 2. 导入到 containerd
sudo ctr --namespace k8s.io images import /tmp/jaeger-mysql-plugin.tar

# 3. 验证
sudo crictl images | grep jaeger-mysql-plugin

# 4. 清理
rm /tmp/jaeger-mysql-plugin.tar
```

### 查看镜像

**使用 crictl**（推荐）:
```bash
# Lima VM
limactl shell k3s-vm sudo crictl images

# 本地
sudo crictl images
```

**使用 ctr**:
```bash
# Lima VM
limactl shell k3s-vm sudo ctr --namespace k8s.io images ls

# 本地
sudo ctr --namespace k8s.io images ls
```

### 删除镜像

```bash
# 使用 crictl
sudo crictl rmi jaeger-mysql-plugin:latest

# 使用 ctr
sudo ctr --namespace k8s.io images rm docker.io/library/jaeger-mysql-plugin:latest
```

## ⚠️ 常见问题

### 问题 1: ImagePullBackOff

**症状**: Pod 状态显示 `ImagePullBackOff` 或 `ErrImagePull`

**原因**: 镜像不在 containerd 中，或 `imagePullPolicy` 设置错误

**解决**:
```bash
# 1. 检查镜像是否存在
sudo crictl images | grep jaeger-mysql-plugin

# 2. 如果不存在，重新导入
./build-and-deploy.sh

# 3. 确认 YAML 中的 imagePullPolicy
kubectl get deployment jaeger-mysql-plugin -n tracing -o yaml | grep imagePullPolicy
# 应该是: imagePullPolicy: Never
```

### 问题 2: 镜像导入后找不到

**症状**: `crictl images` 看不到刚导入的镜像

**原因**: namespace 不正确

**解决**: 确保使用 `--namespace k8s.io`
```bash
# 正确
sudo ctr --namespace k8s.io images import /tmp/image.tar

# 错误（默认 namespace 是 default）
sudo ctr images import /tmp/image.tar
```

### 问题 3: 权限问题

**症状**: `permission denied` 错误

**原因**: crictl 和 ctr 需要 root 权限

**解决**: 使用 `sudo`
```bash
sudo crictl images
sudo ctr --namespace k8s.io images ls
```

## 📊 验证清单

部署后，验证以下内容：

### ✅ 1. 镜像存在
```bash
sudo crictl images | grep jaeger-mysql-plugin
# 应该看到: jaeger-mysql-plugin latest
```

### ✅ 2. Pods 运行
```bash
kubectl get pods -n tracing
# 所有 Pods 应该是 Running 状态
```

### ✅ 3. 镜像拉取策略
```bash
kubectl get deployment jaeger-mysql-plugin -n tracing -o jsonpath='{.spec.template.spec.containers[0].imagePullPolicy}'
# 应该输出: Never
```

### ✅ 4. 没有镜像拉取错误
```bash
kubectl describe pod -n tracing -l app=jaeger-mysql-plugin | grep -i image
# 不应该看到 "Failed to pull image" 或 "ImagePullBackOff"
```

## 🔧 调试命令

### 查看 Pod 事件
```bash
kubectl describe pod -n tracing <pod-name>
```

### 查看 Pod 日志
```bash
kubectl logs -n tracing <pod-name>
```

### 进入 Pod 调试
```bash
kubectl exec -it -n tracing <pod-name> -- sh
```

### 查看镜像详情
```bash
sudo crictl inspecti jaeger-mysql-plugin:latest
```

## 📚 containerd 文档

- [containerd 官方文档](https://containerd.io/)
- [crictl 用户指南](https://github.com/kubernetes-sigs/cri-tools/blob/master/docs/crictl.md)
- [K3s containerd 配置](https://docs.k3s.io/advanced#configuring-containerd)

## 🎯 最佳实践

1. **使用脚本部署**: `./build-and-deploy.sh` 已经适配 containerd
2. **验证镜像**: 部署前使用 `./verify-containerd.sh` 检查
3. **正确的 namespace**: 始终使用 `--namespace k8s.io`
4. **imagePullPolicy**: 本地镜像使用 `Never` 或 `IfNotPresent`
5. **清理旧镜像**: 重新部署前删除旧镜像避免混淆

## 🚀 快速参考

```bash
# 完整部署
./build-and-deploy.sh

# 验证镜像
./verify-containerd.sh

# 查看所有镜像
limactl shell k3s-vm sudo crictl images

# 查看 Pods
kubectl get pods -n tracing

# 查看日志
kubectl logs -n tracing -l app=jaeger-mysql-plugin -f
```

---

**注意**: `build-and-deploy.sh` 已经完全适配 containerd 环境，可以直接使用！🚀



