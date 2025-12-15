#!/bin/bash

# 在 Lima VM 中设置永久 PATH，确保 /usr/local/go/bin 优先

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}╔══════════════════════════════════════════╗"
echo "║   设置 Go PATH (永久)                ║"
echo -e "╚══════════════════════════════════════════╝${NC}"
echo ""

# 检查是否在 Linux 环境中
if [[ "$(uname -s)" == "Darwin" ]]; then
    echo -e "${RED}✗ 您现在在 macOS 宿主机上${NC}"
    echo ""
    echo "请先进入 Lima VM："
    echo -e "${YELLOW}  limactl shell k3s-vm${NC}"
    echo ""
    echo "然后在 VM 中运行此脚本："
    echo -e "${YELLOW}  cd /Users/tal/dock/goutils/k3s/lianlu/jaeger-mysql-plugin${NC}"
    echo -e "${YELLOW}  ./setup-go-path.sh${NC}"
    exit 1
fi

echo -e "${GREEN}✓ 在 Linux 环境中${NC}"
echo ""

# 检测 shell 类型
if [ -n "$ZSH_VERSION" ]; then
    SHELL_RC="$HOME/.zshrc"
    SHELL_NAME="zsh"
elif [ -n "$BASH_VERSION" ]; then
    SHELL_RC="$HOME/.bashrc"
    SHELL_NAME="bash"
    # 如果 .bashrc 不存在，尝试 .bash_profile
    if [ ! -f "$SHELL_RC" ] && [ -f "$HOME/.bash_profile" ]; then
        SHELL_RC="$HOME/.bash_profile"
    fi
else
    SHELL_RC="$HOME/.profile"
    SHELL_NAME="sh"
fi

echo -e "${YELLOW}检测到 Shell: $SHELL_NAME${NC}"
echo -e "${YELLOW}配置文件: $SHELL_RC${NC}"
echo ""

# 创建配置文件（如果不存在）
if [ ! -f "$SHELL_RC" ]; then
    echo -e "${YELLOW}创建配置文件: $SHELL_RC${NC}"
    touch "$SHELL_RC"
fi

# 检查是否已经设置了 PATH
GO_PATH_LINE='export PATH=/usr/local/go/bin:$PATH'

if grep -q "/usr/local/go/bin" "$SHELL_RC"; then
    echo -e "${YELLOW}发现现有的 Go PATH 设置，正在更新...${NC}"
    
    # 删除所有包含 /usr/local/go/bin 的行
    sed -i.bak '/\/usr\/local\/go\/bin/d' "$SHELL_RC"
    
    # 添加新的 PATH 设置（放在最前面）
    echo "" >> "$SHELL_RC"
    echo "# Go 1.21.5 - 优先使用 /usr/local/go/bin" >> "$SHELL_RC"
    echo "$GO_PATH_LINE" >> "$SHELL_RC"
    
    echo -e "${GREEN}✓ 已更新 PATH 设置${NC}"
else
    echo -e "${YELLOW}添加 Go PATH 设置...${NC}"
    
    # 添加注释和 PATH 设置
    echo "" >> "$SHELL_RC"
    echo "# Go 1.21.5 - 优先使用 /usr/local/go/bin" >> "$SHELL_RC"
    echo "$GO_PATH_LINE" >> "$SHELL_RC"
    
    echo -e "${GREEN}✓ 已添加 PATH 设置${NC}"
fi

# 立即应用设置
export PATH=/usr/local/go/bin:$PATH

echo ""
echo -e "${BLUE}=========================================="
echo "验证设置"
echo -e "==========================================${NC}"

# 验证 Go 版本
if command -v go &> /dev/null; then
    GO_VERSION=$(go version | awk '{print $3}')
    GO_PATH=$(which go)
    echo -e "${GREEN}✓ Go 版本: $GO_VERSION${NC}"
    echo -e "${GREEN}✓ Go 路径: $GO_PATH${NC}"
    
    # 检查是否是 /usr/local/go/bin/go
    if [[ "$GO_PATH" == "/usr/local/go/bin/go" ]]; then
        echo -e "${GREEN}✓ 正在使用 /usr/local/go/bin/go (正确)${NC}"
    else
        echo -e "${YELLOW}⚠ 当前使用的 Go 路径: $GO_PATH${NC}"
        echo -e "${YELLOW}  请重新打开终端或运行: source $SHELL_RC${NC}"
    fi
else
    echo -e "${RED}✗ 未找到 Go 命令${NC}"
    echo ""
    echo "请确保 Go 已安装到 /usr/local/go"
    echo "如果未安装，请运行: ./INSTALL_GO_IN_VM.sh"
fi

echo ""
echo -e "${GREEN}╔══════════════════════════════════════════╗"
echo "║          PATH 设置完成！              ║"
echo -e "╚══════════════════════════════════════════╝${NC}"
echo ""
echo "配置文件已更新: $SHELL_RC"
echo ""
echo "要使设置立即生效，请运行："
echo -e "${YELLOW}  source $SHELL_RC${NC}"
echo ""
echo "或者重新打开终端窗口。"
echo ""

