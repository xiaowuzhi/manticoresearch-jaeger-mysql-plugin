#!/bin/bash

# Jaeger + ManticoreSearch 部署管理工具
# 功能：部署、清理、重新部署

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

show_menu() {
    echo -e "${BLUE}=========================================="
    echo "Jaeger + ManticoreSearch 部署管理"
    echo -e "==========================================${NC}"
    echo ""
    echo "1) 完整部署（首次部署）"
    echo "2) 重新部署（保留数据）"
    echo "3) 完全清理并重新部署"
    echo "4) 仅清理（删除所有资源）"
    echo "5) 查看状态"
    echo "0) 退出"
    echo ""
}

deploy_full() {
    echo -e "${BLUE}=========================================="
    echo "开始完整部署"
    echo -e "==========================================${NC}"
    echo ""
    
    # 检查是否已存在
    if kubectl get namespace tracing &>/dev/null; then
        echo -e "${YELLOW}警告: tracing 命名空间已存在${NC}"
        read -p "是否继续？这将更新现有配置 [y/N]: " continue
        if [ "$continue" != "y" ] && [ "$continue" != "Y" ]; then
            echo "取消部署"
            return
        fi
    fi
    
    # 1. 创建命名空间
    echo -e "${YELLOW}步骤 1/3: 创建命名空间...${NC}"
    kubectl apply -f 01-namespace.yaml
    echo -e "${GREEN}✓ 完成${NC}"
    echo ""
    
    # 2. 部署 ManticoreSearch
    echo -e "${YELLOW}步骤 2/3: 部署 ManticoreSearch...${NC}"
    kubectl apply -f 02-manticore.yaml
    echo ""
    echo "等待 ManticoreSearch 就绪（最多 3 分钟）..."
    kubectl wait --for=condition=ready pod -l app=manticore -n tracing --timeout=180s 2>/dev/null || echo "继续..."
    echo -e "${GREEN}✓ 完成${NC}"
    echo ""
    
    # 3. 部署 Jaeger
    echo -e "${YELLOW}步骤 3/3: 部署 Jaeger...${NC}"
    kubectl apply -f 03-jaeger-clean.yaml
    echo ""
    echo "等待 Jaeger 就绪（最多 3 分钟）..."
    kubectl wait --for=condition=ready pod -l component=collector -n tracing --timeout=180s 2>/dev/null || echo "继续..."
    kubectl wait --for=condition=ready pod -l component=query -n tracing --timeout=180s 2>/dev/null || echo "继续..."
    echo -e "${GREEN}✓ 完成${NC}"
    echo ""
    
    # 显示状态
    echo -e "${BLUE}=========================================="
    echo "部署完成！"
    echo -e "==========================================${NC}"
    echo ""
    kubectl get all -n tracing
    echo ""
    echo -e "${GREEN}访问 Jaeger UI:${NC}"
    echo "  http://localhost:30686"
    echo ""
}

redeploy() {
    echo -e "${YELLOW}重新部署 Jaeger 组件（保留 ManticoreSearch 数据）...${NC}"
    echo ""
    
    # 删除 Jaeger 但保留 ManticoreSearch
    kubectl delete deployment jaeger-collector jaeger-query -n tracing 2>/dev/null || true
    kubectl delete daemonset jaeger-agent -n tracing 2>/dev/null || true
    
    echo "等待删除完成..."
    sleep 5
    
    # 重新部署 Jaeger
    kubectl apply -f 03-jaeger-clean.yaml
    
    echo ""
    echo "等待 Jaeger 就绪..."
    kubectl wait --for=condition=ready pod -l app=jaeger -n tracing --timeout=180s 2>/dev/null || true
    
    echo ""
    echo -e "${GREEN}✓ 重新部署完成${NC}"
    echo ""
}

clean_and_redeploy() {
    echo -e "${RED}警告: 这将删除所有数据并重新部署${NC}"
    read -p "确认继续？[y/N]: " confirm
    
    if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
        echo "取消操作"
        return
    fi
    
    echo ""
    echo -e "${YELLOW}删除 tracing 命名空间...${NC}"
    kubectl delete namespace tracing 2>/dev/null || true
    
    echo "等待删除完成..."
    sleep 10
    
    # 重新部署
    deploy_full
}

cleanup() {
    echo -e "${RED}警告: 这将删除所有资源和数据${NC}"
    read -p "确认删除？[y/N]: " confirm
    
    if [ "$confirm" != "y" ] && [ "$confirm" != "Y" ]; then
        echo "取消操作"
        return
    fi
    
    echo ""
    echo -e "${YELLOW}删除 tracing 命名空间...${NC}"
    kubectl delete namespace tracing
    
    echo ""
    echo -e "${GREEN}✓ 清理完成${NC}"
}

show_status() {
    echo -e "${BLUE}=========================================="
    echo "当前状态"
    echo -e "==========================================${NC}"
    echo ""
    
    if ! kubectl get namespace tracing &>/dev/null; then
        echo -e "${YELLOW}tracing 命名空间不存在${NC}"
        echo "运行部署: ./jaeger-deploy.sh"
        return
    fi
    
    echo "📦 Pods:"
    kubectl get pods -n tracing
    echo ""
    
    echo "🌐 Services:"
    kubectl get svc -n tracing
    echo ""
    
    echo "💾 存储:"
    kubectl get pvc -n tracing
    echo ""
    
    # 检查 Collector 配置
    COLLECTOR_POD=$(kubectl get pod -n tracing -l component=collector -o jsonpath='{.items[0].metadata.name}' 2>/dev/null)
    if [ -n "$COLLECTOR_POD" ]; then
        echo "⚙️  存储配置:"
        STORAGE_TYPE=$(kubectl get pod -n tracing -l component=collector -o jsonpath='{.items[0].spec.containers[0].env[?(@.name=="SPAN_STORAGE_TYPE")].value}')
        echo "  SPAN_STORAGE_TYPE: $STORAGE_TYPE"
        
        if [ "$STORAGE_TYPE" = "elasticsearch" ]; then
            ES_URLS=$(kubectl get pod -n tracing -l component=collector -o jsonpath='{.items[0].spec.containers[0].env[?(@.name=="ES_SERVER_URLS")].value}')
            echo "  ES_SERVER_URLS: $ES_URLS"
        fi
    fi
    echo ""
}

# 主菜单
while true; do
    show_menu
    read -p "请选择 [0-5]: " choice
    echo ""
    
    case $choice in
        1)
            deploy_full
            ;;
        2)
            redeploy
            ;;
        3)
            clean_and_redeploy
            ;;
        4)
            cleanup
            ;;
        5)
            show_status
            ;;
        0)
            echo "退出"
            exit 0
            ;;
        *)
            echo -e "${RED}无效选择${NC}"
            ;;
    esac
    
    echo ""
    read -p "按 Enter 继续..."
    clear
done



