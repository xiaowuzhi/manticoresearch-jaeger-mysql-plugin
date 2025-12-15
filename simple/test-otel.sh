#!/bin/bash

# OpenTelemetry OTLP 测试脚本

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}=========================================="
echo "OpenTelemetry OTLP 测试"
echo -e "==========================================${NC}"
echo ""

# 检查当前目录
if [ ! -f "otel_tracer_test.go" ]; then
    echo -e "${RED}错误: 请在 simple 目录中运行此脚本${NC}"
    exit 1
fi

# 检查 kubectl
if ! command -v kubectl &> /dev/null; then
    echo -e "${YELLOW}警告: kubectl 未安装，将使用本地模式${NC}"
    KUBECTL_AVAILABLE=false
else
    KUBECTL_AVAILABLE=true
fi

# 检查 Jaeger 部署
if [ "$KUBECTL_AVAILABLE" = true ]; then
    echo -e "${YELLOW}检查 Jaeger 部署...${NC}"
    if kubectl get pods -n tracing -l component=collector &> /dev/null; then
        COLLECTOR_POD=$(kubectl get pod -n tracing -l component=collector -o jsonpath='{.items[0].metadata.name}')
        if [ -n "$COLLECTOR_POD" ]; then
            echo -e "${GREEN}✓ Jaeger Collector 运行中: $COLLECTOR_POD${NC}"
            
            # 检查存储配置
            STORAGE_TYPE=$(kubectl get pod -n tracing -l component=collector -o jsonpath='{.items[0].spec.containers[0].env[?(@.name=="SPAN_STORAGE_TYPE")].value}')
            echo -e "${GREEN}  存储类型: $STORAGE_TYPE${NC}"
            
            if [ "$STORAGE_TYPE" = "elasticsearch" ]; then
                ES_URLS=$(kubectl get pod -n tracing -l component=collector -o jsonpath='{.items[0].spec.containers[0].env[?(@.name=="ES_SERVER_URLS")].value}')
                echo -e "${GREEN}  存储地址: $ES_URLS${NC}"
            fi
        fi
    else
        echo -e "${RED}✗ Jaeger 未部署，请先运行: cd ../k3s && ./deploy.sh${NC}"
        exit 1
    fi
    echo ""
fi

# 安装依赖
echo -e "${YELLOW}步骤 1/4: 检查依赖...${NC}"
if ! go mod verify &> /dev/null; then
    echo "  下载依赖..."
    go mod tidy
fi
echo -e "${GREEN}✓ 依赖检查完成${NC}"
echo ""

# 设置端口转发
if [ "$KUBECTL_AVAILABLE" = true ]; then
    echo -e "${YELLOW}步骤 2/4: 设置端口转发...${NC}"
    
    # 检查是否已有端口转发
    if lsof -Pi :4317 -sTCP:LISTEN -t >/dev/null 2>&1; then
        echo -e "${GREEN}✓ 端口 4317 已在使用（可能已有端口转发）${NC}"
    else
        echo "  启动端口转发 (后台运行)..."
        kubectl port-forward -n tracing svc/jaeger-collector 4317:4317 > /dev/null 2>&1 &
        PF_PID=$!
        echo "  端口转发 PID: $PF_PID"
        
        # 等待端口转发就绪
        sleep 2
        
        if lsof -Pi :4317 -sTCP:LISTEN -t >/dev/null 2>&1; then
            echo -e "${GREEN}✓ 端口转发设置成功${NC}"
        else
            echo -e "${RED}✗ 端口转发失败${NC}"
            exit 1
        fi
    fi
    echo ""
else
    echo -e "${YELLOW}步骤 2/4: 跳过端口转发 (kubectl 不可用)${NC}"
    echo ""
fi

# 运行测试
echo -e "${YELLOW}步骤 3/4: 运行 OTLP 测试...${NC}"
echo ""

# 设置环境变量
export OTEL_EXPORTER_OTLP_ENDPOINT="localhost:4317"

# 运行测试
if go test -v -run TestOTEL; then
    echo ""
    echo -e "${GREEN}✓ 所有测试通过${NC}"
else
    echo ""
    echo -e "${RED}✗ 测试失败${NC}"
    TEST_FAILED=true
fi

echo ""

# 清理
if [ -n "$PF_PID" ]; then
    echo -e "${YELLOW}步骤 4/4: 清理...${NC}"
    kill $PF_PID 2>/dev/null || true
    echo -e "${GREEN}✓ 端口转发已关闭${NC}"
    echo ""
fi

# 显示结果
echo -e "${BLUE}=========================================="
echo "测试完成"
echo -e "==========================================${NC}"
echo ""

if [ "$TEST_FAILED" = true ]; then
    echo -e "${RED}部分测试失败，请检查日志${NC}"
    exit 1
fi

echo -e "${GREEN}访问 Jaeger UI 查看追踪数据:${NC}"
echo ""
echo "  NodePort: http://localhost:30686"
echo ""
echo "  或使用端口转发:"
echo "    kubectl port-forward -n tracing svc/jaeger-query 16686:16686"
echo "    open http://localhost:16686"
echo ""

echo -e "${GREEN}在 UI 中搜索以下服务:${NC}"
echo "  • test-service"
echo "  • database-service"
echo "  • http-service"
echo "  • nested-service"
echo "  • error-service"
echo "  • multi-operation-service"
echo "  • custom-attrs-service"
echo "  • long-running-service"
echo ""

echo -e "${BLUE}运行单个测试:${NC}"
echo "  go test -v -run TestOTELBasic"
echo "  go test -v -run TestOTELDatabase"
echo "  go test -v -run TestOTELHTTP"
echo ""

echo -e "${BLUE}运行性能测试:${NC}"
echo "  go test -bench=BenchmarkOTELSpanCreation -benchmem"
echo ""

