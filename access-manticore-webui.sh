#!/bin/bash
# ManticoreSearch Web UI 访问脚本
# 提供两种方式：port-forward 和 NodePort

set -e

NAMESPACE="tracing"
SERVICE="manticore"
HTTP_PORT=9308
NODE_PORT=30908  # 自定义 NodePort 端口

echo "🌐 ManticoreSearch Web UI 访问工具"
echo "=================================="
echo ""

# 检查 ManticoreSearch 是否运行
if ! kubectl get deployment manticore -n $NAMESPACE &>/dev/null; then
    echo "❌ 错误: ManticoreSearch 未部署"
    echo "   请先部署: kubectl apply -f k3s/lianlu/k3s/02-manticore.yaml"
    exit 1
fi

echo "✅ ManticoreSearch 已部署"
echo ""

# 检查 Service 类型
SERVICE_TYPE=$(kubectl get svc $SERVICE -n $NAMESPACE -o jsonpath='{.spec.type}' 2>/dev/null || echo "ClusterIP")

if [ "$SERVICE_TYPE" == "NodePort" ]; then
    echo "📡 方式 1: 通过 NodePort 访问（推荐）"
    NODE_IP=$(kubectl get nodes -o jsonpath='{.items[0].status.addresses[?(@.type=="InternalIP")].address}' 2>/dev/null || echo "192.168.5.15")
    ACTUAL_NODE_PORT=$(kubectl get svc $SERVICE -n $NAMESPACE -o jsonpath='{.spec.ports[?(@.name=="http")].nodePort}' 2>/dev/null)
    
    if [ -n "$ACTUAL_NODE_PORT" ]; then
        echo "   🌍 Web UI: http://${NODE_IP}:${ACTUAL_NODE_PORT}"
        echo "   📊 SQL API: http://${NODE_IP}:${ACTUAL_NODE_PORT}/sql"
        echo "   📋 状态: http://${NODE_IP}:${ACTUAL_NODE_PORT}/status"
        echo ""
        echo "   在浏览器中打开:"
        echo "   open http://${NODE_IP}:${ACTUAL_NODE_PORT}"
    else
        echo "   ⚠️  NodePort 未配置 HTTP 端口"
    fi
else
    echo "📡 方式 1: 通过 NodePort 访问"
    echo "   ⚠️  当前 Service 类型为 ClusterIP，需要先配置 NodePort"
    echo "   运行以下命令配置 NodePort:"
    echo ""
    echo "   kubectl patch svc $SERVICE -n $NAMESPACE -p '{\"spec\":{\"type\":\"NodePort\",\"ports\":[{\"name\":\"http\",\"port\":$HTTP_PORT,\"targetPort\":$HTTP_PORT,\"nodePort\":$NODE_PORT}]}}'"
    echo ""
fi

echo "📡 方式 2: 通过 port-forward 访问"
echo "   1. 运行以下命令启动端口转发（在后台运行）:"
echo ""
echo "   kubectl port-forward -n $NAMESPACE svc/$SERVICE $HTTP_PORT:$HTTP_PORT --address=0.0.0.0 &"
echo ""
echo "   2. 然后在浏览器中访问:"
echo "   http://localhost:$HTTP_PORT"
echo "   http://localhost:$HTTP_PORT/sql"
echo "   http://localhost:$HTTP_PORT/status"
echo ""

# 提供快速启动选项
read -p "是否现在启动 port-forward? (y/n) " -n 1 -r
echo ""
if [[ $REPLY =~ ^[Yy]$ ]]; then
    echo "🚀 启动 port-forward..."
    kubectl port-forward -n $NAMESPACE svc/$SERVICE $HTTP_PORT:$HTTP_PORT --address=0.0.0.0 &
    PF_PID=$!
    echo "   ✅ Port-forward 已启动 (PID: $PF_PID)"
    echo "   🌍 访问地址: http://localhost:$HTTP_PORT"
    echo "   📊 SQL API: http://localhost:$HTTP_PORT/sql"
    echo "   📋 状态: http://localhost:$HTTP_PORT/status"
    echo ""
    echo "   按 Ctrl+C 停止 port-forward"
    echo ""
    
    # 等待用户中断
    trap "kill $PF_PID 2>/dev/null; exit" INT TERM
    wait $PF_PID
fi

echo ""
echo "📝 使用示例:"
echo "   # 查询所有表"
echo "   curl -s 'http://localhost:$HTTP_PORT/sql' -d 'mode=raw&query=SHOW TABLES'"
echo ""
echo "   # 查询数据"
echo "   curl -s 'http://localhost:$HTTP_PORT/sql' -d 'mode=raw&query=SELECT * FROM jaeger_spans LIMIT 10'"
echo ""
echo "   # 查看状态"
echo "   curl -s 'http://localhost:$HTTP_PORT/status'"
echo ""

