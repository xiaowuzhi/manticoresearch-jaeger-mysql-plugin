#!/bin/bash

# 简单的运行脚本

set -e

GREEN='\033[0;32m'
YELLOW='\033[1;33m'
BLUE='\033[0;34m'
NC='\033[0m'

echo -e "${BLUE}=========================================="
echo "简单 Jaeger 追踪示例"
echo -e "==========================================${NC}"
echo ""

# 进入目录
cd "$(dirname "$0")"

# 检查参数
if [ "$1" = "test" ]; then
    echo -e "${YELLOW}运行测试...${NC}"
    go test -v -cover
    exit 0
elif [ "$1" = "bench" ]; then
    echo -e "${YELLOW}运行 Benchmark...${NC}"
    go test -bench=. -benchmem
    exit 0
fi

# 默认运行程序
echo -e "${YELLOW}下载依赖...${NC}"
go mod download
echo -e "${GREEN}✓ 依赖就绪${NC}"
echo ""

echo -e "${YELLOW}运行程序...${NC}"
echo -e "${YELLOW}注意: 需要 Jaeger 在 localhost:6831${NC}"
echo ""

go run main.go

echo ""
echo -e "${GREEN}完成！${NC}"

