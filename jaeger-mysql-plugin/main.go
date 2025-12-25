package main

import (
	"database/sql"
	"flag"
	"fmt"
	"net"
	"os"
	"os/signal"
	"strconv"
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

// ====================
// 环境变量辅助函数
// ====================

func getEnvInt(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

func getEnvDuration(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}

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

	// 配置连接池 - 支持环境变量覆盖
	maxOpenConns := getEnvInt("DB_MAX_OPEN_CONNS", 10)
	maxIdleConns := getEnvInt("DB_MAX_IDLE_CONNS", 5)
	connMaxLifetime := getEnvDuration("DB_CONN_MAX_LIFETIME", 5*time.Minute)
	connMaxIdleTime := getEnvDuration("DB_CONN_MAX_IDLE_TIME", 1*time.Minute)

	db.SetMaxOpenConns(maxOpenConns)
	db.SetMaxIdleConns(maxIdleConns)
	db.SetConnMaxLifetime(connMaxLifetime)
	db.SetConnMaxIdleTime(connMaxIdleTime)

	// 测试连接
	if err := db.Ping(); err != nil {
		logger.Error().Err(err).Msg("Failed to ping MySQL")
		os.Exit(1)
	}

	logger.Info().
		Int("max_open_conns", maxOpenConns).
		Int("max_idle_conns", maxIdleConns).
		Dur("conn_max_lifetime", connMaxLifetime).
		Msg("Successfully connected to MySQL v004")

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

		// 优雅关闭 gRPC 服务器
		grpcServer.GracefulStop()

		// 关闭存储（刷新批量写入缓冲区）
		if err := store.Close(); err != nil {
			logger.Error().Err(err).Msg("Failed to close store")
		}
		logger.Info().Msg("Store closed, all pending writes flushed")
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

	// 创建 ManticoreSearch RT index（支持中文分词）
	// 注意：GROUP BY 只能用于 attribute 字段，不能用于 text 字段
	// CJK 中文分词配置：
	//   - ngram_len = '1': 单字切分，适合中文搜索
	//   - ngram_chars = 'cjk': 对 CJK 字符应用 ngram 分词
	//   - min_word_len = '1': 允许单字搜索
	// 注意：ngram_chars 和 charset_table 不能同时指定相同字符集
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
	) ngram_len='1' ngram_chars='cjk' min_word_len='1'
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
