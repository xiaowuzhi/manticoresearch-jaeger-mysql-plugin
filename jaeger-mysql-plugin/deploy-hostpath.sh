#!/bin/bash

# ============================================================
# Jaeger MySQL 存储插件部署脚本 (hostPath 方式)
# 
# 使用相对路径，可从任意目录运行
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

# 相对路径定义
PLUGIN_DIR="$SCRIPT_DIR"                    # jaeger-mysql-plugin/
K3S_DIR="$PROJECT_ROOT/k3s"                 # k3s/
BINARY_NAME="jaeger-mysql-plugin"

echo -e "${BLUE}╔══════════════════════════════════════════════════════════╗"
echo "║       Jaeger MySQL 存储插件部署 (hostPath 方式)          ║"
echo -e "╚══════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "项目根目录: ${GREEN}$PROJECT_ROOT${NC}"
echo -e "插件目录:   ${GREEN}$PLUGIN_DIR${NC}"
echo -e "K3s 配置:   ${GREEN}$K3S_DIR${NC}"
echo ""

# ============================================================
# 前置检查
# ============================================================

cd "$PLUGIN_DIR"

# 检查二进制文件
if [ ! -f "$BINARY_NAME" ]; then
    echo -e "${RED}✗ 找不到二进制文件: $PLUGIN_DIR/$BINARY_NAME${NC}"
    echo ""
    echo -e "${YELLOW}请先编译:${NC}"
    echo "  cd $PLUGIN_DIR"
    echo "  go build -o $BINARY_NAME ."
    echo ""
    exit 1
fi

# 获取二进制文件大小
BINARY_SIZE=$(ls -lh "$BINARY_NAME" | awk '{print $5}')
echo -e "${GREEN}✓ 二进制文件: $BINARY_NAME ($BINARY_SIZE)${NC}"

# 检查 K3s 配置文件
MANTICORE_YAML="$K3S_DIR/02-manticore.yaml"
JAEGER_YAML="$K3S_DIR/04-jaeger-mysql-storage.yaml"

if [ ! -f "$MANTICORE_YAML" ]; then
    echo -e "${RED}✗ 找不到 ManticoreSearch 配置: $MANTICORE_YAML${NC}"
    exit 1
fi

if [ ! -f "$JAEGER_YAML" ]; then
    echo -e "${RED}✗ 找不到 Jaeger 配置: $JAEGER_YAML${NC}"
    exit 1
fi

echo -e "${GREEN}✓ K3s 配置文件存在${NC}"
echo ""

# ============================================================
# 步骤 1: 创建命名空间
# ============================================================

echo -e "${YELLOW}步骤 1/4: 创建命名空间...${NC}"
kubectl create namespace tracing 2>/dev/null || echo "  命名空间已存在"
echo ""

# ============================================================
# 步骤 2: 部署 ManticoreSearch
# ============================================================

echo -e "${YELLOW}步骤 2/4: 部署 ManticoreSearch...${NC}"
if ! kubectl get deployment manticore -n tracing &>/dev/null; then
    kubectl apply -f "$MANTICORE_YAML"
    echo "  等待 ManticoreSearch Pod 就绪..."
    kubectl wait --for=condition=ready pod -l app=manticore -n tracing --timeout=180s || true
else
    echo "  ManticoreSearch 已部署"
fi
echo ""

# ============================================================
# 步骤 3: 部署 MySQL 存储插件 (hostPath)
# ============================================================

echo -e "${YELLOW}步骤 3/4: 部署 MySQL 存储插件 (使用 hostPath)...${NC}"
echo "  hostPath: $PLUGIN_DIR"

cat <<EOF | kubectl apply -f -
apiVersion: apps/v1
kind: Deployment
metadata:
  name: jaeger-mysql-plugin
  namespace: tracing
  labels:
    app: jaeger-mysql-plugin
spec:
  replicas: 2
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
        - /app/$BINARY_NAME
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
        livenessProbe:
          tcpSocket:
            port: 17271
          initialDelaySeconds: 10
          periodSeconds: 30
        readinessProbe:
          tcpSocket:
            port: 17271
          initialDelaySeconds: 5
          periodSeconds: 10
      volumes:
      - name: binary
        hostPath:
          path: $PLUGIN_DIR
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
  sessionAffinity: None
  ports:
  - port: 17271
    targetPort: 17271
    protocol: TCP
    name: grpc
  selector:
    app: jaeger-mysql-plugin
EOF

echo -e "${GREEN}✓ MySQL 插件配置已应用${NC}"

# 强制重启以加载新的二进制文件
echo "  触发滚动更新..."
kubectl rollout restart deployment/jaeger-mysql-plugin -n tracing
echo -e "${GREEN}✓ MySQL 插件滚动更新已触发${NC}"
echo ""

# ============================================================
# 步骤 4: 部署 Jaeger Collector 和 Query
# ============================================================

echo -e "${YELLOW}步骤 4/4: 部署 Jaeger Collector 和 Query...${NC}"

# 删除旧部署
kubectl delete deployment jaeger-collector jaeger-query -n tracing 2>/dev/null || true
kubectl delete deployment jaeger-collector-grpc jaeger-query-grpc -n tracing 2>/dev/null || true

kubectl apply -f "$JAEGER_YAML"

echo -e "${GREEN}✓ Jaeger 组件已部署${NC}"
echo ""

# ============================================================
# 等待所有 Pod 就绪
# ============================================================

echo "等待 Pod 就绪..."
sleep 5
kubectl wait --for=condition=ready pod -l app=jaeger-mysql-plugin -n tracing --timeout=120s 2>/dev/null || true
kubectl wait --for=condition=ready pod -l component=collector -n tracing --timeout=120s 2>/dev/null || true
kubectl wait --for=condition=ready pod -l component=query -n tracing --timeout=120s 2>/dev/null || true

# ============================================================
# 部署状态
# ============================================================

echo ""
echo -e "${BLUE}══════════════════════════════════════════════════════════"
echo "                        部署状态"
echo -e "══════════════════════════════════════════════════════════${NC}"
echo ""
kubectl get pods -n tracing -o wide
echo ""
kubectl get svc -n tracing

# ============================================================
# 完成提示
# ============================================================

echo ""
echo -e "${GREEN}╔══════════════════════════════════════════════════════════╗"
echo "║                     部署完成！                            ║"
echo -e "╚══════════════════════════════════════════════════════════╝${NC}"
echo ""
echo -e "${BLUE}访问地址:${NC}"
echo "  Jaeger UI:  http://localhost:30686"
echo ""
echo -e "${BLUE}常用命令:${NC}"
echo "  # 查看插件日志"
echo "  kubectl logs -n tracing -l app=jaeger-mysql-plugin -f"
echo ""
echo "  # 查看 Collector 日志"
echo "  kubectl logs -n tracing -l component=collector -f"
echo ""
echo "  # 重新编译并更新"
echo "  cd $PLUGIN_DIR && go build -o $BINARY_NAME . && kubectl rollout restart deployment/jaeger-mysql-plugin -n tracing"
echo ""
