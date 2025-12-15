# Jaeger MySQL Plugin 编译指南

## 🎯 快速编译命令

### 在 Lima VM 中编译（推荐）

```bash
# 1. 进入 VM
limactl shell k3s-vm

# 2. 进入插件目录
cd /Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin

# 3. 设置 Go 环境
export PATH=/usr/local/go/bin:$PATH

# 4. 清理并编译（ARM64）
rm -f jaeger-mysql-plugin go.sum
go mod tidy
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
  -a -installsuffix cgo \
  -ldflags '-w -s' \
  -o jaeger-mysql-plugin .

# 5. 验证编译结果
file jaeger-mysql-plugin
ls -lh jaeger-mysql-plugin
```

### 在 macOS 宿主机编译（交叉编译）

```bash
# 1. 进入插件目录
cd /Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin

# 2. 清理并编译（ARM64 for Linux）
rm -f jaeger-mysql-plugin go.sum
go mod tidy
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
  -a -installsuffix cgo \
  -ldflags '-w -s' \
  -o jaeger-mysql-plugin .

# 3. 验证编译结果
file jaeger-mysql-plugin
# 输出应该显示: ELF 64-bit LSB executable, ARM aarch64
```

---

## 📦 编译参数说明

```bash
CGO_ENABLED=0           # 禁用 CGO，生成静态链接二进制
GOOS=linux              # 目标操作系统：Linux
GOARCH=arm64            # 目标架构：ARM64（K3s 节点架构）
-a                      # 强制重新编译所有包
-installsuffix cgo      # 添加后缀以区分 CGO/非 CGO 构建
-ldflags '-w -s'        # 链接器标志：
                        #   -w: 禁用 DWARF 调试信息
                        #   -s: 禁用符号表
                        # 这两个标志可以显著减小二进制大小
```

---

## 🔄 完整编译和部署流程

### 方法 1：使用自动化脚本

```bash
# 在 Lima VM 中执行
cd /Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin
./deploy-hostpath.sh
```

**脚本会自动执行：**
1. 检查 Go 环境
2. 编译插件（ARM64）
3. 创建 hostPath 目录
4. 部署到 K3s
5. 重启 Plugin Pod

### 方法 2：手动编译和部署

```bash
# 1. 在 Lima VM 中编译
limactl shell k3s-vm
cd /Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin
export PATH=/usr/local/go/bin:$PATH

# 清理旧文件
rm -f jaeger-mysql-plugin go.sum

# 更新依赖
go mod tidy

# 编译
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
  -a -installsuffix cgo \
  -ldflags '-w -s' \
  -o jaeger-mysql-plugin .

# 2. 创建 hostPath 目录并复制二进制
sudo mkdir -p /var/lib/jaeger-plugin
sudo cp jaeger-mysql-plugin /var/lib/jaeger-plugin/
sudo chmod +x /var/lib/jaeger-plugin/jaeger-mysql-plugin

# 3. 应用 Kubernetes 配置
kubectl apply -f ../k3s/04-jaeger-mysql-storage.yaml

# 4. 重启 Plugin Pod
kubectl delete pod -n tracing -l app=jaeger-mysql-plugin

# 5. 验证
kubectl get pods -n tracing -l app=jaeger-mysql-plugin
kubectl logs -n tracing -l app=jaeger-mysql-plugin --tail=20
```

---

## 🐛 编译问题排查

### 问题 1: exec format error

**错误信息：**
```
exec /app/jaeger-mysql-plugin: exec format error
```

**原因：** 编译的架构不匹配（编译为 x86-64 但 K3s 节点是 ARM64）

**解决：**
```bash
# 检查 K3s 节点架构
kubectl get nodes -o wide
# 或
uname -m  # 在 Lima VM 中

# 确保使用正确的 GOARCH
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build ...
```

### 问题 2: Go 版本不匹配

**错误信息：**
```
compile: version "go1.21.5" does not match go tool version "go1.20"
```

**解决：**
```bash
# 清理模块缓存
go clean -modcache
rm -f go.sum

# 重新下载依赖
go mod tidy

# 或者升级 Go 版本
cd /tmp
wget https://go.dev/dl/go1.21.5.linux-arm64.tar.gz
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf go1.21.5.linux-arm64.tar.gz
export PATH=/usr/local/go/bin:$PATH
go version
```

### 问题 3: 依赖下载失败

**错误信息：**
```
go: github.com/xxx: connection timeout
```

**解决：**
```bash
# 设置 Go 代理
export GOPROXY=https://goproxy.cn,direct

# 或使用官方代理
export GOPROXY=https://proxy.golang.org,direct

# 然后重新编译
go mod tidy
```

---

## 📊 编译后的文件大小

```bash
# 标准编译（带调试信息）
# 大小：~25-30 MB

# 使用 -ldflags '-w -s' 优化
# 大小：~12-15 MB

# 查看文件大小
ls -lh jaeger-mysql-plugin
du -h jaeger-mysql-plugin
```

---

## 🔍 验证编译结果

### 1. 检查文件信息

```bash
# 文件类型
file jaeger-mysql-plugin
# 期望输出：ELF 64-bit LSB executable, ARM aarch64, version 1 (SYSV), statically linked

# 文件大小
ls -lh jaeger-mysql-plugin

# 查看依赖（应该是静态链接，无外部依赖）
ldd jaeger-mysql-plugin 2>&1 || echo "Static binary (no dependencies)"
```

### 2. 本地测试运行

```bash
# 查看版本/帮助信息
./jaeger-mysql-plugin --help

# 测试连接（需要 ManticoreSearch 运行）
./jaeger-mysql-plugin \
  --grpc-addr=:17271 \
  --mysql-addr=localhost:9306 \
  --mysql-user=root \
  --mysql-pass=123456
```

---

## 📝 编译环境要求

### 最小要求

- **Go**: 1.18+（推荐 1.21+）
- **磁盘空间**: ~500MB（Go 模块缓存）
- **内存**: 2GB+
- **网络**: 需要访问 Go 模块代理

### 推荐配置

- **Go**: 1.21.5
- **OS**: Linux ARM64 或 macOS ARM64（交叉编译）
- **工具**: make, git

---

## 🚀 一键编译脚本

创建 `quick-build.sh`：

```bash
#!/bin/bash
set -e

echo "🔨 开始编译 jaeger-mysql-plugin..."

# 设置 Go 环境
export PATH=/usr/local/go/bin:$PATH
export GOPROXY=https://goproxy.cn,direct

# 清理
echo "清理旧文件..."
rm -f jaeger-mysql-plugin go.sum

# 更新依赖
echo "更新依赖..."
go mod tidy

# 编译
echo "编译中..."
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build \
  -a -installsuffix cgo \
  -ldflags '-w -s' \
  -o jaeger-mysql-plugin .

# 验证
echo ""
echo "✅ 编译完成！"
echo "文件信息："
file jaeger-mysql-plugin
ls -lh jaeger-mysql-plugin

echo ""
echo "📦 二进制文件: $(pwd)/jaeger-mysql-plugin"
echo ""
echo "下一步："
echo "  1. 部署: sudo cp jaeger-mysql-plugin /var/lib/jaeger-plugin/"
echo "  2. 重启: kubectl delete pod -n tracing -l app=jaeger-mysql-plugin"
```

**使用方法：**
```bash
chmod +x quick-build.sh
./quick-build.sh
```

---

## 🎯 常用命令速查

```bash
# 快速编译（Lima VM）
cd /Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin && \
export PATH=/usr/local/go/bin:$PATH && \
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -ldflags '-w -s' -o jaeger-mysql-plugin .

# 编译并部署
cd /Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin && \
export PATH=/usr/local/go/bin:$PATH && \
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -ldflags '-w -s' -o jaeger-mysql-plugin . && \
sudo cp jaeger-mysql-plugin /var/lib/jaeger-plugin/ && \
kubectl delete pod -n tracing -l app=jaeger-mysql-plugin

# 编译并查看日志
cd /Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin && \
export PATH=/usr/local/go/bin:$PATH && \
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -a -installsuffix cgo -ldflags '-w -s' -o jaeger-mysql-plugin . && \
sudo cp jaeger-mysql-plugin /var/lib/jaeger-plugin/ && \
kubectl delete pod -n tracing -l app=jaeger-mysql-plugin && \
sleep 15 && \
kubectl logs -n tracing -l app=jaeger-mysql-plugin --tail=30
```

---

**📚 相关文档：**
- README.md - 插件完整文档
- deploy-hostpath.sh - 自动化部署脚本
- ../k3s/04-jaeger-mysql-storage.yaml - Kubernetes 配置



