package tlog

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestLogger(t *testing.T) {
	// 初始化
	err := Init(Config{
		ServiceName: "test-service1",
		Endpoint:    "localhost:4317",
	})
	if err != nil {
		t.Logf("Init failed (Jaeger 可能未运行): %v", err)
	}
	defer Shutdown(context.Background())

	// 创建 Span
	ctx, span := Log.Start(context.Background(), "test-operation")
	defer Log.End(span)

	// ========== Log.Info ==========
	t.Run("Info", func(t *testing.T) {
		Log.Info(ctx, "用户登录成功",
			"user_id", 12345,
			"login_type", "password",
			"ip", "192.168.1.1",
		)

		Log.Info(ctx, "订单创建完成", "order_id", "ORD-001", "amount", 99.99)
	})

	// ========== Log.Error ==========
	t.Run("Error", func(t *testing.T) {
		// 方式1: 传入 error
		testErr := errors.New("数据库连接超时")
		Log.Error(ctx, testErr)

		// 方式2: 传入 error + 说明
		Log.Error(ctx, errors.New("库存不足"), "商品ID: SKU-001")

		// 方式3: 格式化错误
		Log.Errorf(ctx, "用户 %d 权限不足，需要 %s 权限", 123, "admin")
	})

	// ========== Log.Tags ==========
	t.Run("Tags", func(t *testing.T) {
		// 单个标签
		Log.Tag(ctx, "order_id", "ORD-123456")

		// 批量标签 (key-value 形式)
		Log.Tags(ctx,
			"env", "production",
			"version", "v1.2.3",
			"region", "cn-shanghai",
			"tenant_id", 1001,
		)

		// Map 形式
		Log.TagsMap(ctx, map[string]interface{}{
			"custom_key1": "value1",
			"custom_key2": 123,
		})

		// 快捷标签
		Log.HTTP(ctx, "POST", "/api/v1/orders", 200)
		Log.User(ctx, "U-12345", "张三")
	})

	// ========== Log.Event ==========
	t.Run("Event", func(t *testing.T) {
		Log.Event(ctx, "order.created",
			"order_id", "ORD-123",
			"amount", 99.99,
			"items", 3,
		)

		Log.Event(ctx, "payment.success",
			"channel", "alipay",
			"transaction_id", "TX-789",
		)
	})

	// ========== Log.Warn / Log.Debug ==========
	t.Run("WarnAndDebug", func(t *testing.T) {
		Log.Warn(ctx, "缓存即将过期", "key", "user:123", "ttl", "60s")
		Log.Debug(ctx, "请求详情", map[string]string{"header": "xxx", "body": "yyy"})
	})

	// ========== Log.SQL ==========
	t.Run("SQL", func(t *testing.T) {
		Log.SQL(ctx, "SELECT * FROM users WHERE id = ?", "12ms")
		Log.SQL(ctx, "INSERT INTO orders (user_id, amount) VALUES (?, ?)", "5ms")
	})

	time.Sleep(100 * time.Millisecond)
	t.Log("测试完成")
}

// 业务使用示例
func TestLoggerExampleUsage(t *testing.T) {
	// ===== 1. 应用启动时初始化 =====
	Init(Config{
		ServiceName: "order-service2",
		Endpoint:    "localhost:30317",
	})
	defer Shutdown(context.Background())

	// ===== 2. 处理请求时 =====
	ctx, span := Log.Start(context.Background(), "CreateOrder")
	defer Log.End(span)

	// 设置标签
	Log.HTTP(ctx, "POST", "/api/orders", 200)
	Log.User(ctx, "12345", "test-user")
	Log.Tags(ctx, "env", "prod", "version", "v1.0")

	// 记录日志
	Log.Info(ctx, "开始创建订单", "order_type", "normal")

	// 记录 SQL
	Log.SQL(ctx, "INSERT INTO orders ...", "5ms")

	// 如果出错
	Log.Error(ctx, errors.New("创建订单失败"))

	// 添加事件
	Log.Event(ctx, "order.created", "order_id", "ORD-001", "amount", 99.99)
}


