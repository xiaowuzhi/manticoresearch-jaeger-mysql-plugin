package tlog

import (
	"context"
	"errors"
	"fmt"
	"testing"
	"time"
)

func TestBasicUsage(t *testing.T) {
	// 初始化
	err := Init(Config{
		ServiceName: "test-service",
		Endpoint:    "localhost:30317",
	})
	if err != nil {
		t.Logf("Init warning (collector may not be running): %v", err)
	}
	defer Shutdown(context.Background())

	// 创建 Span
	ctx, span := Log.Start(context.Background(), "test-operation")
	defer Log.End(span)

	// 验证 TraceID
	if tid := TraceID(ctx); tid != "" {
		t.Logf("TraceID: %s", tid)
	}

	// 基本日志
	Log.Info(ctx, "测试消息", "key", "value")
	Log.Warn(ctx, "警告消息")
	Log.Debug(ctx, "调试消息", "data", map[string]any{"a": 1})
	Log.Error(ctx, errors.New("测试错误"))
	Log.Errorf(ctx, "格式化错误: %d", 123)

	// 标签
	Log.Tag(ctx, "env", "test")
	Log.Tags(ctx, "k1", "v1", "k2", 123)
	Log.HTTP(ctx, "POST", "/api/test", 200)
	Log.User(ctx, "U001", "测试用户")

	// 事件
	Log.Event(ctx, "order.created", "order_id", "ORD-001", "amount", 99.99)
	Log.SQL(ctx, "SELECT * FROM users", "5ms")

	time.Sleep(100 * time.Millisecond)
	t.Log("基本测试完成")
}

// TestChineseContent 测试中文内容（验证中文分词）
func TestChineseContent(t *testing.T) {
	Init(Config{
		ServiceName: "chinese-test",
		Endpoint:    "localhost:30317", // Jaeger NodePort
	})
	defer Shutdown(context.Background())

	ctx, span := Log.Start(context.Background(), "中文测试")
	defer Log.End(span)

	// 中文日志 - 这些内容会被发送到 Jaeger，然后存储到 ManticoreSearch
	// ManticoreSearch 使用 ngram_chars='cjk' 进行中文分词
	Log.Info(ctx, "用户登录成功",
		"用户名", "张三",
		"城市", "北京",
		"操作", "密码登录",
	)

	Log.Info(ctx, "订单创建完成",
		"订单号", "ORD-20231225-001",
		"商品", "iPhone 15 Pro",
		"金额", 9999.00,
		"收货地址", "上海市浦东新区",
	)

	Log.Warn(ctx, "库存不足警告",
		"商品", "MacBook Pro",
		"当前库存", 5,
		"预警阈值", 10,
	)

	Log.Error(ctx, errors.New("支付失败：余额不足"),
		"用户", "李四",
		"金额", 1000.00,
	)

	// 中文标签
	Log.Tags(ctx,
		"服务", "订单服务",
		"环境", "生产环境",
		"版本", "v1.2.3",
	)

	// 中文事件
	Log.Event(ctx, "支付成功",
		"支付方式", "微信支付",
		"交易号", "WX20231225001",
		"金额", 99.99,
	)

	Log.SQL(ctx, "SELECT * FROM orders WHERE user_id = '张三'", "12ms")

	time.Sleep(200 * time.Millisecond)
	t.Log("中文测试完成 - 请检查 Jaeger UI 和 ManticoreSearch")
}

// TestSearchableContent 生成可搜索的测试数据
func TestSearchableContent(t *testing.T) {
	Init(Config{
		ServiceName: "search-test",
		Endpoint:    "localhost:30317",
	})
	defer Shutdown(context.Background())

	// 生成多条测试数据，便于在 ManticoreSearch 中验证中文搜索
	testCases := []struct {
		operation string
		msg       string
		tags      map[string]any
	}{
		{
			operation: "北京用户登录",
			msg:       "北京用户张三登录成功",
			tags:      map[string]any{"城市": "北京", "用户": "张三"},
		},
		{
			operation: "上海订单创建",
			msg:       "上海用户李四创建订单",
			tags:      map[string]any{"城市": "上海", "用户": "李四"},
		},
		{
			operation: "深圳支付完成",
			msg:       "深圳用户王五支付成功",
			tags:      map[string]any{"城市": "深圳", "用户": "王五"},
		},
		{
			operation: "广州发货通知",
			msg:       "广州仓库已发货",
			tags:      map[string]any{"城市": "广州", "状态": "已发货"},
		},
		{
			operation: "杭州退款处理",
			msg:       "杭州用户申请退款",
			tags:      map[string]any{"城市": "杭州", "类型": "退款"},
		},
	}

	for _, tc := range testCases {
		ctx, span := Log.Start(context.Background(), tc.operation)
		Log.Info(ctx, tc.msg, tc.tags)
		Log.Tags(ctx, "operation", tc.operation)
		Log.End(span)
	}

	time.Sleep(500 * time.Millisecond)

	// 使用 fmt.Println 确保日志始终显示
	fmt.Println("\n========================================")
	fmt.Println("✅ 搜索测试数据已生成")
	fmt.Println("========================================")
	fmt.Println("")
	fmt.Println("📊 Jaeger UI:")
	fmt.Println("   http://localhost:30686")
	fmt.Println("")
	fmt.Println("🔍 ManticoreSearch 查询:")
	fmt.Println("   方法1: 打开 manticore-query.html")
	fmt.Println("   方法2: mysql -h localhost -P 31306")
	fmt.Println("   方法3: curl -X POST http://localhost:30399/sql -d \"query=SHOW TABLES\"")
	fmt.Println("")
	fmt.Println("📝 示例 SQL:")
	fmt.Println("   SELECT * FROM jaeger_spans WHERE MATCH('北京');")
	fmt.Println("   SELECT * FROM jaeger_spans WHERE MATCH('订单');")
	fmt.Println("   SELECT * FROM jaeger_spans WHERE MATCH('张三');")
	fmt.Println("========================================")
}

// Example 展示标准使用方式
func Example() {
	// 1. 应用启动时初始化
	Init(Config{
		ServiceName: "my-service",
		Endpoint:    "jaeger-collector:30317",
	})
	defer Shutdown(context.Background())

	// 2. 处理请求时创建 Span
	ctx, span := Log.Start(context.Background(), "HandleRequest")
	defer Log.End(span)

	// 3. 记录日志
	Log.Info(ctx, "处理请求", "path", "/api/users")

	// 4. 设置标签
	Log.HTTP(ctx, "GET", "/api/users", 200)
	Log.User(ctx, "123", "test-user")

	// 5. 记录事件
	Log.Event(ctx, "cache.hit", "key", "user:123")

	// 6. 记录 SQL
	Log.SQL(ctx, "SELECT * FROM users WHERE id = 123", "3ms")

	// 7. 错误处理
	if false {
		Log.Error(ctx, errors.New("something went wrong"))
	}
}
