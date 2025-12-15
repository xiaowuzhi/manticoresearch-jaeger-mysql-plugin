#!/bin/bash

# 快速部署 ManticoreSearch（用于测试修复）

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

echo -e "${BLUE}=========================================="
echo "ManticoreSearch 快速部署"
echo -e "==========================================${NC}"
echo ""

# 检查命名空间
if ! kubectl get namespace tracing &>/dev/null; then
    echo "创建 tracing 命名空间..."
    kubectl create namespace tracing
    echo ""
fi

# 删除旧部署
echo -e "${YELLOW}步骤 1: 清理旧部署...${NC}"
kubectl delete deployment manticore -n tracing 2>/dev/null && echo "  ✓ 删除 deployment" || echo "  - deployment 不存在"
kubectl delete configmap manticore-config -n tracing 2>/dev/null && echo "  ✓ 删除 configmap" || echo "  - configmap 不存在"
kubectl delete service manticore -n tracing 2>/dev/null && echo "  ✓ 删除 service" || echo "  - service 不存在"

# 不删除 PVC，保留数据
echo "  - 保留 PVC (manticore-data)"
echo ""

sleep 3

# 部署
echo -e "${YELLOW}步骤 2: 部署 ManticoreSearch...${NC}"
kubectl apply -f 02-manticore.yaml
echo ""

# 等待
echo -e "${YELLOW}步骤 3: 等待 Pod 启动...${NC}"
echo "等待 20 秒..."
for i in {20..1}; do
    echo -n "$i..."
    sleep 1
done
echo ""
echo ""

# 获取 Pod 名称
POD_NAME=$(kubectl get pod -n tracing -l app=manticore -o jsonpath='{.items[0].metadata.name}' 2>/dev/null || echo "")

if [ -z "$POD_NAME" ]; then
    echo -e "${RED}✗ 未找到 ManticoreSearch Pod${NC}"
    echo ""
    echo "检查部署状态："
    kubectl get pods -n tracing
    exit 1
fi

echo -e "${GREEN}✓ Pod: $POD_NAME${NC}"
echo ""

# 查看状态
echo -e "${YELLOW}步骤 4: 检查 Pod 状态...${NC}"
kubectl get pod $POD_NAME -n tracing -o wide
echo ""

POD_STATUS=$(kubectl get pod $POD_NAME -n tracing -o jsonpath='{.status.phase}')
POD_READY=$(kubectl get pod $POD_NAME -n tracing -o jsonpath='{.status.containerStatuses[0].ready}')

echo "状态: $POD_STATUS"
echo "就绪: $POD_READY"
echo ""

# 查看 initContainer 日志
echo -e "${YELLOW}步骤 5: initContainer 日志...${NC}"
kubectl logs $POD_NAME -n tracing -c init-config 2>/dev/null || echo "(已完成或无日志)"
echo ""

# 查看容器日志
echo -e "${YELLOW}步骤 6: ManticoreSearch 日志...${NC}"
kubectl logs $POD_NAME -n tracing -c manticore --tail=30 2>/dev/null || echo "(容器可能还在启动)"
echo ""

# 检查错误
if kubectl logs $POD_NAME -n tracing -c manticore 2>/dev/null | grep -i "fatal\|error"; then
    echo -e "${RED}⚠ 发现错误日志${NC}"
    echo ""
else
    echo -e "${GREEN}✓ 没有发现错误${NC}"
    echo ""
fi

# 测试连接
if [ "$POD_STATUS" = "Running" ] && [ "$POD_READY" = "true" ]; then
    echo -e "${YELLOW}步骤 7: 测试连接...${NC}"
    
    echo "测试 HTTP 端口 (9308):"
    if kubectl exec $POD_NAME -n tracing -- sh -c "wget -q -O- http://localhost:9308/ 2>/dev/null" &>/dev/null; then
        echo -e "${GREEN}✓ HTTP 端口正常${NC}"
    else
        echo -e "${RED}✗ HTTP 端口无响应${NC}"
    fi
    
    echo ""
    echo "测试 MySQL 端口 (9306):"
    if kubectl exec $POD_NAME -n tracing -- sh -c "nc -zv localhost 9306 2>&1" | grep -q "open"; then
        echo -e "${GREEN}✓ MySQL 端口正常${NC}"
    else
        echo -e "${RED}✗ MySQL 端口无响应${NC}"
    fi
    echo ""
fi

# 总结
echo -e "${BLUE}=========================================="
echo "部署结果"
echo -e "==========================================${NC}"
echo ""

if [ "$POD_STATUS" = "Running" ] && [ "$POD_READY" = "true" ]; then
    echo -e "${GREEN}✓✓✓ ManticoreSearch 部署成功！✓✓✓${NC}"
    echo ""
    echo "服务地址:"
    echo "  HTTP: manticore.tracing.svc.cluster.local:9308"
    echo "  MySQL: manticore.tracing.svc.cluster.local:9306"
    echo ""
    echo "下一步:"
    echo "  1. 部署 Jaeger: kubectl apply -f 03-jaeger-clean.yaml"
    echo "  2. 或使用管理工具: ./jaeger.sh"
    echo ""
elif [ "$POD_STATUS" = "Running" ]; then
    echo -e "${YELLOW}⚠ Pod 正在运行但尚未就绪${NC}"
    echo ""
    echo "等待更多时间或查看日志："
    echo "  kubectl logs -n tracing $POD_NAME -c manticore -f"
    echo ""
elif [ "$POD_STATUS" = "Pending" ]; then
    echo -e "${YELLOW}⚠ Pod 等待调度${NC}"
    echo ""
    kubectl describe pod $POD_NAME -n tracing | tail -20
    echo ""
else
    echo -e "${RED}✗ 部署失败${NC}"
    echo ""
    echo "诊断信息："
    kubectl describe pod $POD_NAME -n tracing | tail -30
    echo ""
fi

echo "实时日志："
echo "  kubectl logs -n tracing $POD_NAME -c manticore -f"
echo ""



