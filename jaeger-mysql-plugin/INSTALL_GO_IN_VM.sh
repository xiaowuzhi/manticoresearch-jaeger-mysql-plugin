#!/bin/bash

# 在 Lima K3s VM 中安装 Go 并构建 Jaeger MySQL 插件

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}╔══════════════════════════════════════════╗"
echo "║   在 Lima VM 中安装 Go 并构建插件      ║"
echo -e "╚══════════════════════════════════════════╝${NC}"
echo ""

# 检查是否在 Lima VM 中
if [[ "$(uname -s)" == "Darwin" ]]; then
    echo -e "${RED}✗ 您现在在 macOS 宿主机上${NC}"
    echo ""
    echo "请先进入 Lima VM："
    echo -e "${YELLOW}  limactl shell k3s-vm${NC}"
    echo ""
    echo "然后在 VM 中运行此脚本："
    echo -e "${YELLOW}  cd /Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin${NC}"
    echo -e "${YELLOW}  ./INSTALL_GO_IN_VM.sh${NC}"
    exit 1
fi

echo -e "${GREEN}✓ 在 Linux 环境中${NC}"
echo ""

# 检查 Go 是否已安装
if command -v go &> /dev/null; then
    echo -e "${GREEN}✓ Go 已安装: $(go version)${NC}"
    echo ""
    
    # 直接运行构建脚本
    echo -e "${YELLOW}开始构建...${NC}"
    exec ./build-without-docker.sh
    exit 0
fi

echo -e "${YELLOW}步骤 1: 安装 Go...${NC}"

# 方式 1: 使用 apt（Ubuntu/Debian）
if command -v apt &> /dev/null; then
    echo "使用 apt 安装 Go..."
    sudo apt update
    sudo apt install -y golang-go
    
    # 验证安装
    if command -v go &> /dev/null; then
        echo -e "${GREEN}✓ Go 安装成功: $(go version)${NC}"
        echo ""
        
        # 运行构建脚本
        echo -e "${YELLOW}步骤 2: 开始构建...${NC}"
        exec ./build-without-docker.sh
        exit 0
    fi
fi

# 方式 2: 手动下载安装（备用）
echo "尝试手动下载安装..."

GO_VERSION="1.21.5"
GO_FILE="go${GO_VERSION}.linux-amd64.tar.gz"
GO_URL="https://go.dev/dl/${GO_FILE}"

# 尝试使用国内镜像
echo "尝试从 goproxy.cn 下载..."
wget -O /tmp/${GO_FILE} https://goproxy.cn/golang.org/dl/${GO_FILE} || \
wget -O /tmp/${GO_FILE} ${GO_URL}

if [ ! -f "/tmp/${GO_FILE}" ]; then
    echo -e "${RED}✗ 下载失败${NC}"
    echo ""
    echo "请手动安装 Go："
    echo "  sudo apt update && sudo apt install -y golang-go"
    exit 1
fi

# 安装
echo "安装 Go..."
sudo rm -rf /usr/local/go
sudo tar -C /usr/local -xzf /tmp/${GO_FILE}

# 设置环境变量（优先使用 /usr/local/go/bin）
if ! grep -q "/usr/local/go/bin" ~/.bashrc; then
    echo '' >> ~/.bashrc
    echo '# Go 1.21.5 - 优先使用 /usr/local/go/bin' >> ~/.bashrc
    echo 'export PATH=/usr/local/go/bin:$PATH' >> ~/.bashrc
else
    # 如果已存在，更新为优先版本
    sed -i.bak 's|export PATH=.*/usr/local/go/bin.*|export PATH=/usr/local/go/bin:$PATH|g' ~/.bashrc
fi

export PATH=/usr/local/go/bin:$PATH

# 清理
rm -f /tmp/${GO_FILE}

# 验证
if command -v go &> /dev/null; then
    echo -e "${GREEN}✓ Go 安装成功: $(go version)${NC}"
    echo ""
    
    # 运行构建脚本
    echo -e "${YELLOW}步骤 2: 开始构建...${NC}"
    source ~/.bashrc
    exec ./build-without-docker.sh
else
    echo -e "${RED}✗ 安装失败${NC}"
    exit 1
fi


