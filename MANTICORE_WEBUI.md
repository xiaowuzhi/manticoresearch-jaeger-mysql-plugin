# 🌐 ManticoreSearch Web UI 访问指南

## 📋 概述

ManticoreSearch 本身没有内置的图形化 Web UI，但提供了强大的 HTTP API（端口 9308），可以通过浏览器或命令行工具访问。

## 🚀 快速访问

### 方法 1: 使用访问脚本（推荐）

```bash
cd /Users/tal/dock/goutils/k3s/lianlu
./access-manticore-webui.sh
```

脚本会自动检测配置并提供访问方式。

---

### 方法 2: Port-Forward（临时访问）

在宿主机或 Lima VM 中运行：

```bash
# 启动 port-forward（后台运行）
kubectl port-forward -n tracing svc/manticore 9308:9308 --address=0.0.0.0 &

# 访问地址
# HTTP API: http://localhost:9308
# SQL API:  http://localhost:9308/sql
# 状态:     http://localhost:9308/status
```

**停止 port-forward:**
```bash
# 查找进程
ps aux | grep "port-forward.*manticore"

# 停止进程
kill <PID>
```

---

### 方法 3: NodePort（永久访问）

#### 3.1 部署 NodePort Service

```bash
# 应用 NodePort 配置
kubectl apply -f k3s/lianlu/k3s/02-manticore-nodeport.yaml

# 查看 NodePort 端口
kubectl get svc manticore-nodeport -n tracing
```

#### 3.2 访问地址

假设 Lima VM IP 是 `192.168.5.15`：

- **HTTP API**: http://192.168.5.15:30908
- **SQL API**: http://192.168.5.15:30908/sql
- **状态**: http://192.168.5.15:30908/status
- **MySQL**: 192.168.5.15:30906

#### 3.3 修改现有 Service 为 NodePort

```bash
# 将现有的 ClusterIP Service 改为 NodePort
kubectl patch svc manticore -n tracing -p '{"spec":{"type":"NodePort","ports":[{"name":"http","port":9308,"targetPort":9308,"nodePort":30908}]}}'
```

---

## 📊 HTTP API 使用示例

### 1. 查看状态

```bash
# 命令行
curl -s 'http://localhost:9308/status'

# 浏览器
# 访问: http://localhost:9308/status
```

### 2. SQL 查询

```bash
# 查看所有表
curl -s 'http://localhost:9308/sql' \
  -d 'mode=raw&query=SHOW TABLES'

# 查询数据
curl -s 'http://localhost:9308/sql' \
  -d 'mode=raw&query=SELECT * FROM jaeger_spans LIMIT 10'

# 统计数量
curl -s 'http://localhost:9308/sql' \
  -d 'mode=raw&query=SELECT COUNT(*) FROM jaeger_spans'
```

### 3. 浏览器访问 SQL API

在浏览器中访问：
```
http://localhost:9308/sql?mode=raw&query=SHOW TABLES
```

或者使用 POST 请求（需要浏览器插件或工具）。

---

## 🛠️ 第三方 Web UI 工具

### 选项 1: ManticoreSearch Adminer

可以使用 Adminer 等 MySQL 管理工具连接 ManticoreSearch：

```bash
# 通过 port-forward 访问 MySQL 端口
kubectl port-forward -n tracing svc/manticore 9306:9306 --address=0.0.0.0 &

# 使用 MySQL 客户端连接
mysql -h 127.0.0.1 -P 9306 -u root
```

### 选项 2: 使用 Postman/Insomnia

配置 HTTP 请求：
- **URL**: `http://localhost:9308/sql`
- **Method**: POST
- **Body**: `mode=raw&query=SHOW TABLES`

### 选项 3: 简单的 HTML 查询页面

创建一个简单的 HTML 页面来查询 ManticoreSearch：

```html
<!DOCTYPE html>
<html>
<head>
    <title>ManticoreSearch Query</title>
</head>
<body>
    <h1>ManticoreSearch SQL Query</h1>
    <form id="queryForm">
        <textarea id="sql" rows="5" cols="80">SHOW TABLES</textarea><br>
        <button type="submit">执行查询</button>
    </form>
    <pre id="result"></pre>
    
    <script>
        document.getElementById('queryForm').onsubmit = async function(e) {
            e.preventDefault();
            const sql = document.getElementById('sql').value;
            const response = await fetch('http://localhost:9308/sql', {
                method: 'POST',
                headers: {'Content-Type': 'application/x-www-form-urlencoded'},
                body: 'mode=raw&query=' + encodeURIComponent(sql)
            });
            const data = await response.json();
            document.getElementById('result').textContent = JSON.stringify(data, null, 2);
        };
    </script>
</body>
</html>
```

---

## 🔍 常用查询

### 查看所有表
```sql
SHOW TABLES
```

### 查看表结构
```sql
DESCRIBE jaeger_spans
```

### 查询数据
```sql
SELECT * FROM jaeger_spans LIMIT 10
```

### 统计查询
```sql
SELECT service_name, COUNT(*) as count 
FROM jaeger_spans 
GROUP BY service_name
```

### 时间范围查询
```sql
SELECT * FROM jaeger_spans 
WHERE start_time > 1700000000000000000 
ORDER BY start_time DESC 
LIMIT 20
```

---

## 📝 端口说明

| 端口 | 协议 | 用途 | 访问方式 |
|------|------|------|----------|
| 9306 | MySQL | MySQL 协议查询 | `mysql -h host -P 9306` |
| 9308 | HTTP | HTTP API / SQL API | `curl http://host:9308/sql` |
| 9312 | Binary | 二进制协议 | 内部使用 |

---

## 🐛 故障排查

### 问题 1: Port-forward 连接被拒绝

```bash
# 检查 Pod 是否运行
kubectl get pods -n tracing -l app=manticore

# 检查 Service
kubectl get svc manticore -n tracing

# 检查端口是否被占用
lsof -i :9308
```

### 问题 2: 无法访问 HTTP API

```bash
# 在 Pod 内部测试
kubectl exec -n tracing deployment/manticore -- \
  curl -s 'http://localhost:9308/status'

# 检查日志
kubectl logs -n tracing -l app=manticore --tail=50
```

### 问题 3: NodePort 无法访问

```bash
# 检查 NodePort 配置
kubectl get svc manticore-nodeport -n tracing -o yaml

# 检查防火墙规则（Lima VM）
# 确保端口已开放
```

---

## ✅ 验证清单

- [ ] ManticoreSearch Pod 运行正常
- [ ] Service 已创建
- [ ] Port-forward 或 NodePort 已配置
- [ ] HTTP API 可以访问 (`/status`)
- [ ] SQL API 可以查询 (`/sql`)

---

## 🔗 相关文档

- [ManticoreSearch HTTP API 文档](https://manual.manticoresearch.com/Connecting_to_ManticoreSearch/HTTP_API)
- [ManticoreSearch SQL 语法](https://manual.manticoresearch.com/SQL)
- [快速启动指南](./QUICKSTART.md)
- [完整集成指南](../../MANTICORESEARCH_INTEGRATION.md)

