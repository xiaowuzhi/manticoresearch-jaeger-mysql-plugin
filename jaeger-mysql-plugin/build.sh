#!/bin/bash

# 构建 Jaeger MySQL Plugin
# 确保使用 Go 1.21.5 (位于 /usr/local/go/bin/go)

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo -e "${BLUE}╔══════════════════════════════════════════╗"
echo "║  构建 Jaeger MySQL 存储插件            ║"
echo -e "╚══════════════════════════════════════════╝${NC}"
echo ""

cd "$SCRIPT_DIR"

# 检查并设置 Go 环境
GO_BIN="/usr/local/go/bin/go"
if [ ! -f "$GO_BIN" ]; then
    echo -e "${RED}✗ 找不到 Go 1.21.5: $GO_BIN${NC}"
    echo "请确保已安装 Go 1.21.5 到 /usr/local/go"
    exit 1
fi

# 使用 Go 1.21.5
export PATH="/usr/local/go/bin:$PATH"

# 检查 Go 版本
GO_VERSION=$($GO_BIN version | awk '{print $3}')
echo -e "${GREEN}✓ 使用 Go 版本: $GO_VERSION${NC}"

# 检查版本是否符合要求 (go.mod 要求 1.21+)
if [[ "$GO_VERSION" < "go1.21" ]]; then
    echo -e "${RED}✗ Go 版本过低: $GO_VERSION，需要 go1.21+${NC}"
    exit 1
fi

echo ""
echo -e "${YELLOW}步骤 1/3: 清理旧文件...${NC}"
rm -f jaeger-mysql-plugin go.sum
echo -e "${GREEN}✓ 清理完成${NC}"
echo ""

echo -e "${YELLOW}步骤 2/3: 更新依赖...${NC}"
$GO_BIN mod tidy
echo -e "${GREEN}✓ 依赖更新完成${NC}"
echo ""

echo -e "${YELLOW}步骤 3/3: 编译二进制文件 (ARM64 Linux)...${NC}"
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 $GO_BIN build \
  -a -installsuffix cgo \
  -ldflags '-w -s' \
  -o jaeger-mysql-plugin .

if [ ! -f "jaeger-mysql-plugin" ]; then
    echo -e "${RED}✗ 编译失败${NC}"
    exit 1
fi

echo -e "${GREEN}✓ 编译成功${NC}"
echo ""

# 显示文件信息
echo -e "${BLUE}=========================================="
echo "编译结果"
echo -e "==========================================${NC}"
file jaeger-mysql-plugin
ls -lh jaeger-mysql-plugin | awk '{print "文件大小: " $5}'
echo ""

echo -e "${GREEN}╔══════════════════════════════════════════╗"
echo "║          构建完成！                     ║"
echo -e "╚══════════════════════════════════════════╝${NC}"
echo ""
echo "二进制文件: $SCRIPT_DIR/jaeger-mysql-plugin"
echo ""
echo "下一步："
echo "  运行部署脚本: ./deploy-hostpath.sh"
echo "  或手动部署: kubectl apply -f ../k3s/04-jaeger-mysql-storage.yaml"
echo ""

