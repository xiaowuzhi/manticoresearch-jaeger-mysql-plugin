#!/bin/bash

# 使用 hostPath 直接挂载二进制文件

set -e

GREEN='\033[0;32m'
BLUE='\033[0;34m'
YELLOW='\033[1;33m'
RED='\033[0;31m'
NC='\033[0m'

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"

echo -e "${BLUE}╔══════════════════════════════════════════╗"
echo "║  部署 Jaeger MySQL 存储插件            ║"
echo "║      (hostPath 方式)                    ║"
echo -e "╚══════════════════════════════════════════╝${NC}"
echo ""

cd "$SCRIPT_DIR"

# 检查二进制
if [ ! -f "jaeger-mysql-plugin" ]; then
    echo -e "${RED}✗ 找不到二进制文件${NC}"
    exit 1
fi

BINARY_PATH=$(readlink -f jaeger-mysql-plugin)
echo -e "${GREEN}✓ 二进制文件: $BINARY_PATH ($(ls -lh jaeger-mysql-plugin | awk '{print $5}'))${NC}"
echo ""

echo -e "${YELLOW}步骤 1/4: 创建命名空间...${NC}"
kubectl create namespace tracing 2>/dev/null || echo "  命名空间已存在"
echo ""

echo -e "${YELLOW}步骤 2/4: 部署 ManticoreSearch...${NC}"
if ! kubectl get deployment manticore -n tracing &>/dev/null; then
    kubectl apply -f ../k3s/02-manticore.yaml
    kubectl wait --for=condition=ready pod -l app=manticore -n tracing --timeout=180s || true
else
    echo "  ManticoreSearch 已部署"
fi
echo ""

echo -e "${YELLOW}步骤 3/4: 部署 MySQL 存储插件 (使用 hostPath)...${NC}"

cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jaeger-mysql-plugin
  namespace: tracing
  labels:
    app: jaeger-mysql-plugin
spec:
  replicas: 1
  selector:
    matchLabels:
      app: jaeger-mysql-plugin
  template:
    metadata:
      labels:
        app: jaeger-mysql-plugin
    spec:
      containers:
      - name: plugin
        image: alpine:latest
        command:
        - /app/jaeger-mysql-plugin
        args:
        - --grpc-addr=:17271
        - --mysql-addr=manticore:9306
        - --mysql-db=jaeger
        - --mysql-user=root
        - --mysql-pass=
        ports:
        - containerPort: 17271
          name: grpc
        volumeMounts:
        - name: binary
          mountPath: /app
          readOnly: true
        resources:
          requests:
            memory: "128Mi"
            cpu: "100m"
          limits:
            memory: "512Mi"
            cpu: "500m"
      volumes:
      - name: binary
        hostPath:
          path: $SCRIPT_DIR
          type: Directory
---
apiVersion: v1
kind: Service
metadata:
  name: jaeger-mysql-plugin
  namespace: tracing
  labels:
    app: jaeger-mysql-plugin
spec:
  type: ClusterIP
  # 对于 gRPC，设置 sessionAffinity: None 以确保多副本负载均衡
  # gRPC 使用 HTTP/2 长连接，默认会保持会话亲和性，可能导致流量只到一个 pod
  sessionAffinity: None
  ports:
  - port: 17271
    targetPort: 17271
    protocol: TCP
    name: grpc
  selector:
    app: jaeger-mysql-plugin
EOF

echo -e "${GREEN}✓ MySQL 插件已部署${NC}"
echo ""

echo -e "${YELLOW}步骤 4/4: 部署 Jaeger Collector 和 Query...${NC}"

# 删除旧部署
kubectl delete deployment jaeger-collector jaeger-query -n tracing 2>/dev/null || true
kubectl delete deployment jaeger-collector-grpc jaeger-query-grpc -n tracing 2>/dev/null || true

kubectl apply -f ../k3s/04-jaeger-mysql-storage.yaml

echo -e "${GREEN}✓ Jaeger 组件已部署${NC}"
echo ""

echo "等待 Pod 就绪..."
sleep 5
kubectl wait --for=condition=ready pod -l app=jaeger-mysql-plugin -n tracing --timeout=120s || true
kubectl wait --for=condition=ready pod -l component=collector -n tracing --timeout=120s || true
kubectl wait --for=condition=ready pod -l component=query -n tracing --timeout=120s || true

echo ""
echo -e "${BLUE}=========================================="
echo "部署状态"
echo -e "==========================================${NC}"
kubectl get pods -n tracing -o wide
echo ""
kubectl get svc -n tracing

echo ""
echo -e "${GREEN}╔══════════════════════════════════════════╗"
echo "║          部署完成！                     ║"
echo -e "╚══════════════════════════════════════════╝${NC}"
echo ""

echo "访问 Jaeger UI:"
echo "  http://localhost:30686"
echo ""

echo "查看插件日志:"
echo "  kubectl logs -n tracing -l app=jaeger-mysql-plugin -f"
echo ""

echo "查看 Collector 日志:"
echo "  kubectl logs -n tracing -l component=collector -f"
echo ""


