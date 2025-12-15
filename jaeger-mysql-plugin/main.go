package main

import (
	"database/sql"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"github.com/jaegertracing/jaeger/plugin/storage/grpc/shared"
	"github.com/rs/zerolog"
	"google.golang.org/grpc"
)

var (
	grpcAddr  = flag.String("grpc-addr", ":17271", "gRPC server address")
	mysqlAddr = flag.String("mysql-addr", "manticore:9306", "MySQL/ManticoreSearch address")
	mysqlDB   = flag.String("mysql-db", "jaeger", "MySQL database name")
	mysqlUser = flag.String("mysql-user", "root", "MySQL username")
	mysqlPass = flag.String("mysql-pass", "", "MySQL password")
)

func main() {
	flag.Parse()

	// 增加文件描述符限制，避免 "too many open files" 错误
	var rlimit syscall.Rlimit
	if err := syscall.Getrlimit(syscall.RLIMIT_NOFILE, &rlimit); err == nil {
		// 设置更高的文件描述符限制
		rlimit.Cur = 1048576
		if rlimit.Max < 1048576 {
			rlimit.Max = 1048576
		}
		if err := syscall.Setrlimit(syscall.RLIMIT_NOFILE, &rlimit); err != nil {
			// 如果设置失败，记录警告但继续运行
			fmt.Fprintf(os.Stderr, "Warning: failed to set file descriptor limit: %v\n", err)
		} else {
			fmt.Fprintf(os.Stderr, "File descriptor limit set to: %d\n", rlimit.Cur)
		}
	}

	// 使用 zerolog，轻量级日志库，避免文件监听器
	zerolog.TimeFieldFormat = zerolog.TimeFormatUnix
	zerolog.SetGlobalLevel(zerolog.DebugLevel) // 使用 Info 级别减少日志输出
	logger := zerolog.New(os.Stderr).With().
		Str("module", "jaeger-mysql-plugin").
		Timestamp().
		Logger()

	// 连接 MySQL/ManticoreSearch
	// interpolateParams=true: 客户端插值，避免使用服务端预处理语句（ManticoreSearch 不支持）
	var dsn string
	if *mysqlDB != "" {
		dsn = fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&multiStatements=true&interpolateParams=true",
			*mysqlUser, *mysqlPass, *mysqlAddr, *mysqlDB)
	} else {
		dsn = fmt.Sprintf("%s:%s@tcp(%s)/?parseTime=true&multiStatements=true&interpolateParams=true",
			*mysqlUser, *mysqlPass, *mysqlAddr)
	}

	logger.Info().Str("dsn", dsn).Msg("Connecting to MySQL")

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to connect to MySQL")
		os.Exit(1)
	}
	defer db.Close()

	// 配置连接池以避免 "too many open files" 错误
	// 限制最大打开连接数（进一步降低以减少文件描述符使用）
	db.SetMaxOpenConns(3)
	// 设置最大空闲连接数
	db.SetMaxIdleConns(1)
	// 设置连接最大生存时间（2分钟，更短以更快释放连接）
	db.SetConnMaxLifetime(2 * time.Minute)
	// 设置连接最大空闲时间（15秒，更短以更快释放连接）
	db.SetConnMaxIdleTime(15 * time.Second)

	// 测试连接
	if err := db.Ping(); err != nil {
		logger.Error().Err(err).Msg("Failed to ping MySQL")
		os.Exit(1)
	}

	logger.Info().Int("max_open_conns", 3).Int("max_idle_conns", 1).Msg("Successfully connected to MySQL")

	// 初始化数据库表
	if err := initDatabase(db, logger); err != nil {
		logger.Error().Err(err).Msg("Failed to initialize database")
		os.Exit(1)
	}

	// 创建存储插件
	store := NewMySQLStore(db, logger)

	// 启动 gRPC server
	listener, err := net.Listen("tcp", *grpcAddr)
	if err != nil {
		logger.Error().Err(err).Msg("Failed to listen")
		os.Exit(1)
	}

	// 配置 gRPC 服务器选项，限制并发连接和流
	grpcServer := grpc.NewServer(
		grpc.MaxConcurrentStreams(100), // 限制每个连接的最大并发流
	)

	// 使用 shared.StorageGRPCPlugin 包装 store
	plugin := &shared.StorageGRPCPlugin{
		Impl: store,
	}
	plugin.GRPCServer(nil, grpcServer)

	logger.Info().Str("address", *grpcAddr).Msg("Starting gRPC server")

	// 处理信号
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt, syscall.SIGTERM)

	go func() {
		<-sigChan
		logger.Info().Msg("Shutting down...")
		grpcServer.GracefulStop()
	}()

	if err := grpcServer.Serve(listener); err != nil {
		logger.Error().Err(err).Msg("Failed to serve")
		os.Exit(1)
	}
}

func initDatabase(db *sql.DB, logger zerolog.Logger) error {
	logger.Info().Msg("Initializing database tables...")

	// ManticoreSearch 不需要 CREATE DATABASE 和 USE 语句
	// 跳过数据库创建和选择步骤

	// 创建 ManticoreSearch RT index（简化版）
	// 注意：GROUP BY 只能用于 attribute 字段，不能用于 text 字段
	createTableSQL := `
	CREATE TABLE IF NOT EXISTS jaeger_spans (
		trace_id string attribute,
		span_id string attribute,
		operation_name string attribute,
		flags int,
		start_time bigint,
		duration bigint,
		tags text,
		logs text,
		refs text,
		process text,
		service_name string attribute
	)
	`

	var err error
	_, err = db.Exec(createTableSQL)
	if err != nil {
		logger.Warn().Err(err).Msg("Failed to create table (may already exist or syntax not supported)")
		// ManticoreSearch 可能已有表或语法略有不同，我们尝试继续
	}

	logger.Info().Msg("Database initialization complete")
	return nil
}
