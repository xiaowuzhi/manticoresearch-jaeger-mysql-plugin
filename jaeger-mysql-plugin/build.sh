#!/bin/bash

# ============================================================
# Jaeger MySQL 存储插件构建脚本
# 
# 使用相对路径，可从任意目录运行
# 支持自动检测平台或交叉编译
# ============================================================

set -e

# 颜色定义
GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

# ============================================================
# 路径计算 - 基于脚本位置自动推导
# ============================================================

# 当前脚本所在目录 (jaeger-mysql-plugin/)
SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

# 项目根目录 (manticore-jaeger-mysql-plugin/)
PROJECT_ROOT="$(cd "$SCRIPT_DIR/.." && pwd)"

# 输出文件名
BINARY_NAME="jaeger-mysql-plugin"

# ============================================================
# 解析命令行参数
# ============================================================

TARGET_OS=""
TARGET_ARCH=""
CLEAN_BUILD=false

usage() {
    echo "用法: $0 [选项]"
    echo ""
    echo "选项:"
    echo "  --linux-arm64    交叉编译 Linux ARM64 (适用于 K3s on ARM)"
    echo "  --linux-amd64    交叉编译 Linux AMD64 (适用于 K3s on x86)"
    echo "  --native         编译当前平台 (默认)"
    echo "  --clean          清理后重新构建"
    echo "  -h, --help       显示帮助"
    echo ""
    exit 0
}

while [[ $# -gt 0 ]]; do
    case $1 in
        --linux-arm64)
            TARGET_OS="linux"
            TARGET_ARCH="arm64"
            shift
            ;;
        --linux-amd64)
            TARGET_OS="linux"
            TARGET_ARCH="amd64"
            shift
            ;;
        --native)
            TARGET_OS=""
            TARGET_ARCH=""
            shift
            ;;
        --clean)
            CLEAN_BUILD=true
            shift
            ;;
        -h|--help)
            usage
            ;;
        *)
            echo -e "${RED}未知选项: $1${NC}"
            usage
            ;;
    esac
done

# ============================================================
# 开始构建
# ============================================================

echo -e "${BLUE}╔══════════════════════════════════════════════════════════╗"
echo "║          Jaeger MySQL 存储插件构建                        ║"
echo -e "╚══════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "项目根目录: ${GREEN}$PROJECT_ROOT${NC}"
echo -e "插件目录:   ${GREEN}$SCRIPT_DIR${NC}"
echo ""

cd "$SCRIPT_DIR"

# ============================================================
# 检查 Go 环境
# ============================================================

# 优先使用 /usr/local/go/bin/go，否则使用 PATH 中的 go
if [ -f "/usr/local/go/bin/go" ]; then
    GO_BIN="/usr/local/go/bin/go"
    export PATH="/usr/local/go/bin:$PATH"
elif command -v go &>/dev/null; then
    GO_BIN="$(command -v go)"
else
    echo -e "${RED}✗ 找不到 Go 编译器${NC}"
    echo "请安装 Go 1.21+ 或设置 PATH"
    exit 1
fi

GO_VERSION=$($GO_BIN version | awk '{print $3}')
echo -e "${GREEN}✓ Go 编译器: $GO_BIN${NC}"
echo -e "${GREEN}✓ Go 版本:   $GO_VERSION${NC}"

# 检查版本是否符合要求 (go.mod 要求 1.21+)
GO_MINOR=$(echo "$GO_VERSION" | sed 's/go1\.\([0-9]*\).*/\1/')
if [[ "$GO_MINOR" -lt 21 ]]; then
    echo -e "${RED}✗ Go 版本过低: $GO_VERSION，需要 go1.21+${NC}"
    exit 1
fi
echo ""

# ============================================================
# 确定目标平台
# ============================================================

if [ -z "$TARGET_OS" ]; then
    # 自动检测当前平台
    TARGET_OS=$(uname -s | tr '[:upper:]' '[:lower:]')
    if [ "$TARGET_OS" = "darwin" ]; then
        TARGET_OS="darwin"
    fi
fi

if [ -z "$TARGET_ARCH" ]; then
    # 自动检测当前架构
    MACHINE=$(uname -m)
    case "$MACHINE" in
        x86_64|amd64)
            TARGET_ARCH="amd64"
            ;;
        arm64|aarch64)
            TARGET_ARCH="arm64"
            ;;
        *)
            TARGET_ARCH="$MACHINE"
            ;;
    esac
fi

echo -e "${BLUE}目标平台: ${TARGET_OS}/${TARGET_ARCH}${NC}"
echo ""

# ============================================================
# 步骤 1: 清理
# ============================================================

echo -e "${YELLOW}步骤 1/3: 清理旧文件...${NC}"

if [ "$CLEAN_BUILD" = true ]; then
    rm -f "$BINARY_NAME" go.sum
    echo "  已删除: $BINARY_NAME, go.sum"
elif [ -f "$BINARY_NAME" ]; then
    rm -f "$BINARY_NAME"
    echo "  已删除: $BINARY_NAME"
else
    echo "  无需清理"
fi
echo ""

# ============================================================
# 步骤 2: 更新依赖
# ============================================================

echo -e "${YELLOW}步骤 2/3: 更新依赖...${NC}"
$GO_BIN mod tidy
echo -e "${GREEN}✓ 依赖更新完成${NC}"
echo ""

# ============================================================
# 步骤 3: 编译
# ============================================================

echo -e "${YELLOW}步骤 3/3: 编译二进制文件...${NC}"

# 设置编译环境变量
export CGO_ENABLED=0
export GOOS="$TARGET_OS"
export GOARCH="$TARGET_ARCH"

# 编译
$GO_BIN build \
    -a -installsuffix cgo \
    -ldflags '-w -s' \
    -o "$BINARY_NAME" .

if [ ! -f "$BINARY_NAME" ]; then
    echo -e "${RED}✗ 编译失败${NC}"
    exit 1
fi

echo -e "${GREEN}✓ 编译成功${NC}"
echo ""

# ============================================================
# 显示结果
# ============================================================

echo -e "${BLUE}══════════════════════════════════════════════════════════"
echo "                        编译结果"
echo -e "══════════════════════════════════════════════════════════${NC}"
echo ""
file "$BINARY_NAME"
BINARY_SIZE=$(ls -lh "$BINARY_NAME" | awk '{print $5}')
echo -e "文件大小: ${GREEN}$BINARY_SIZE${NC}"
echo ""

echo -e "${GREEN}╔══════════════════════════════════════════════════════════╗"
echo "║                     构建完成！                            ║"
echo -e "╚══════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}输出文件:${NC}"
echo "  $SCRIPT_DIR/$BINARY_NAME"
echo ""
echo -e "${BLUE}下一步:${NC}"
echo "  # 部署到 K8s"
echo "  ./deploy-hostpath.sh"
echo ""
echo "  # 或重新编译其他平台"
echo "  ./build.sh --linux-arm64   # K3s on ARM (树莓派等)"
echo "  ./build.sh --linux-amd64   # K3s on x86"
echo "  ./build.sh --native        # 当前平台"
echo ""
