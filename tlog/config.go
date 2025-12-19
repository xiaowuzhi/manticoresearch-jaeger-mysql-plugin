package tlog

import (
	"io"
	"time"

	"github.com/rs/zerolog"
)

type Config struct {
	ServiceName string // 服务名称
	Endpoint    string // OTLP gRPC 端点，如 "localhost:4317"
	HostName    string // 主机名

	// Console 控制是否把日志同时打印到控制台；nil 表示默认开启
	Console *bool
	// CheckConn 控制 Init 时是否进行一次 gRPC 连接探测；nil 表示默认开启
	CheckConn *bool
	// FailFast 控制当连接探测失败时是否直接返回 error；nil 表示默认不失败（只打印错误）
	FailFast *bool
	// ConnectTimeout 探测超时；0 表示默认 1s
	ConnectTimeout time.Duration

	// ZLogger：如果你想完全自定义 zerolog（输出、hook、格式等），直接传入（优先级最高）
	ZLogger *zerolog.Logger
	// ZWriter：未传 ZLogger 时使用；可以用 io.MultiWriter(...) 实现“上报 + 控制台”同时输出
	ZWriter io.Writer
	// ZLevel：未传 ZLogger 时使用；默认 Info
	ZLevel zerolog.Level
}
