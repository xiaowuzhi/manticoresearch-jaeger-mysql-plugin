package main

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	jsoniter "github.com/json-iterator/go"

	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/storage/dependencystore"
	"github.com/jaegertracing/jaeger/storage/spanstore"
	"github.com/rs/zerolog"
)

// json-iterator: 比标准库快 2-3 倍，完全兼容
var json = jsoniter.ConfigCompatibleWithStandardLibrary

// ====================
// 配置（支持环境变量）
// ====================

var (
	// 服务列表缓存过期时间
	servicesCacheTTL = getDurationEnv("CACHE_TTL", 30*time.Second)
	// 操作列表缓存过期时间
	operationsCacheTTL = getDurationEnv("CACHE_TTL", 30*time.Second)
	// 批量写入缓冲区大小
	batchWriteSize = getIntEnv("BATCH_SIZE", 50)
	// 批量写入超时时间
	batchWriteTimeout = getDurationEnv("BATCH_TIMEOUT", 500*time.Millisecond)
)

// getIntEnv 获取整数环境变量
func getIntEnv(key string, defaultVal int) int {
	if v := os.Getenv(key); v != "" {
		if i, err := strconv.Atoi(v); err == nil {
			return i
		}
	}
	return defaultVal
}

// getDurationEnv 获取时间环境变量
func getDurationEnv(key string, defaultVal time.Duration) time.Duration {
	if v := os.Getenv(key); v != "" {
		if d, err := time.ParseDuration(v); err == nil {
			return d
		}
	}
	return defaultVal
}

// ====================
// sync.Pool 复用对象
// ====================

// argsSlice 包装切片以满足 sync.Pool 的指针要求
type argsSlice struct {
	data []interface{}
}

// argsPool 复用 INSERT 参数切片
var argsPool = sync.Pool{
	New: func() interface{} {
		return &argsSlice{
			data: make([]interface{}, 0, 550), // 50 spans * 11 fields
		}
	},
}

// ====================
// ====================
// 缓存结构
// ====================

type cacheEntry[T any] struct {
	data      T
	expiresAt time.Time
}

func (c *cacheEntry[T]) isValid() bool {
	return time.Now().Before(c.expiresAt)
}

// ====================
// MySQLStore 主结构
// ====================

type MySQLStore struct {
	db     *sql.DB
	logger zerolog.Logger

	// 缓存
	servicesCache   *cacheEntry[[]string]
	operationsCache map[string]*cacheEntry[[]spanstore.Operation]
	cacheMu         sync.RWMutex

	// 批量写入
	spanBuffer chan *model.Span
	stopCh     chan struct{}
	stopped    bool // 标记是否已停止
	stopMu     sync.RWMutex
	wg         sync.WaitGroup
}

func NewMySQLStore(db *sql.DB, logger zerolog.Logger) *MySQLStore {
	store := &MySQLStore{
		db:              db,
		logger:          logger,
		operationsCache: make(map[string]*cacheEntry[[]spanstore.Operation]),
		spanBuffer:      make(chan *model.Span, batchWriteSize*2),
		stopCh:          make(chan struct{}),
	}

	// 启动批量写入 goroutine
	store.wg.Add(1)
	go store.batchWriteLoop()

	return store
}

// Close 关闭存储，刷新缓冲区
func (s *MySQLStore) Close() error {
	s.stopMu.Lock()
	if s.stopped {
		s.stopMu.Unlock()
		return nil
	}
	s.stopped = true
	s.stopMu.Unlock()

	close(s.stopCh)
	s.wg.Wait()
	return nil
}

// isStopped 检查是否已停止
func (s *MySQLStore) isStopped() bool {
	s.stopMu.RLock()
	defer s.stopMu.RUnlock()
	return s.stopped
}

// batchWriteLoop 批量写入循环
func (s *MySQLStore) batchWriteLoop() {
	defer s.wg.Done()

	batch := make([]*model.Span, 0, batchWriteSize)
	ticker := time.NewTicker(batchWriteTimeout)
	defer ticker.Stop()

	flush := func() {
		if len(batch) == 0 {
			return
		}
		if err := s.writeBatch(context.Background(), batch); err != nil {
			s.logger.Error().Err(err).Int("count", len(batch)).Msg("Failed to write batch")
		} else {
			s.logger.Debug().Int("count", len(batch)).Msg("Batch write completed")
		}
		batch = batch[:0]
	}

	for {
		select {
		case span := <-s.spanBuffer:
			if span == nil {
				continue
			}
			batch = append(batch, span)
			if len(batch) >= batchWriteSize {
				flush()
			}
		case <-ticker.C:
			flush()
		case <-s.stopCh:
			// 关闭前，先 drain 缓冲区中的剩余数据
		drainLoop:
			for {
				select {
				case span := <-s.spanBuffer:
					if span != nil {
						batch = append(batch, span)
					}
				default:
					break drainLoop
				}
			}
			flush()
			return
		}
	}
}

// writeBatch 批量写入 spans
func (s *MySQLStore) writeBatch(ctx context.Context, spans []*model.Span) error {
	if len(spans) == 0 {
		return nil
	}

	// 构建批量 INSERT 语句（预分配空间，减少扩容）
	var sb strings.Builder
	sb.Grow(len(spans) * 256) // 每个 span 约 200-256 字节
	sb.WriteString(`INSERT INTO jaeger_spans (
		trace_id, span_id, operation_name, flags,
		start_time, duration, tags, logs, refs, process, service_name
	) VALUES `)

	// 使用 sync.Pool 复用 args 切片，减少 GC 压力
	as := argsPool.Get().(*argsSlice)
	as.data = as.data[:0] // 重置但保留底层数组

	for i, span := range spans {
		if i > 0 {
			sb.WriteString(", ")
		}
		sb.WriteString("(?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)")

		// 类型专用序列化（无反射，高性能）
		as.data = append(as.data,
			span.TraceID.String(),
			span.SpanID.String(),
			span.OperationName,
			span.Flags,
			span.StartTime.UnixNano(),
			span.Duration.Nanoseconds(),
			marshalTags(span.Tags),
			marshalLogs(span.Logs),
			marshalRefs(span.References),
			marshalProcess(span.Process),
			span.Process.ServiceName,
		)
	}

	_, err := s.db.ExecContext(ctx, sb.String(), as.data...)

	// 归还到 pool（即使出错也要归还）
	argsPool.Put(as)

	if err != nil {
		return fmt.Errorf("batch insert failed: %w", err)
	}

	// 写入后清除服务缓存（可能有新服务）
	s.invalidateServicesCache()

	return nil
}

// ============================================================
// 类型专用的 JSON 序列化函数（无反射，高性能）
// ============================================================

// marshalTags 序列化 tags 字段
func marshalTags(tags []model.KeyValue) string {
	if len(tags) == 0 {
		return "[]"
	}
	return encodeJSON(tags)
}

// marshalLogs 序列化 logs 字段
func marshalLogs(logs []model.Log) string {
	if len(logs) == 0 {
		return "[]"
	}
	return encodeJSON(logs)
}

// marshalRefs 序列化 refs 字段
func marshalRefs(refs []model.SpanRef) string {
	if len(refs) == 0 {
		return "[]"
	}


	logger := zerolog.New(os.Stderr).With().
		Str("module", "refs").
		Timestamp().
		Logger()

		str := encodeJSON(refs)
		logger.Debug().Msg(fmt.Sprintf("====>001 refs %#v str %#v", refs, str))

	return str
}

// marshalProcess 序列化 process 字段
func marshalProcess(p *model.Process) string {
	if p == nil {
		return "{}"
	}
	return encodeJSON(p)
}

// encodeJSON 通用 JSON 编码
// 使用标准库 json.Marshal，简洁且性能足够
// 如需更高性能，可替换为 github.com/json-iterator/go 或 github.com/bytedance/sonic
func encodeJSON(v interface{}) string {
	data, err := json.Marshal(v)
	if err != nil {
		return "[]"
	}

	logger := zerolog.New(os.Stderr).With().
		Str("module", "refs1").
		Timestamp().
		Logger()
	logger.Debug().Msg(fmt.Sprintf("====>002 data %#v", data))

	return string(data)
}

// invalidateServicesCache 使服务缓存失效
func (s *MySQLStore) invalidateServicesCache() {
	s.cacheMu.Lock()
	s.servicesCache = nil
	s.cacheMu.Unlock()
}

// 实现 StoragePluginServer 接口
func (s *MySQLStore) SpanReader() spanstore.Reader {
	return &MySQLSpanReader{
		store:  s,
		db:     s.db,
		logger: s.logger,
	}
}

func (s *MySQLStore) SpanWriter() spanstore.Writer {
	return &MySQLSpanWriter{
		store:  s,
		db:     s.db,
		logger: s.logger,
	}
}

func (s *MySQLStore) DependencyReader() dependencystore.Reader {
	return &MySQLDependencyReader{
		db:     s.db,
		logger: s.logger,
	}
}

// ====================
// SpanWriter 实现
// ====================

type MySQLSpanWriter struct {
	store  *MySQLStore
	db     *sql.DB
	logger zerolog.Logger
}

func (w *MySQLSpanWriter) WriteSpan(ctx context.Context, span *model.Span) error {
	w.logger.Debug().
		Str("trace_id", span.TraceID.String()).
		Str("span_id", span.SpanID.String()).
		Msg("Queuing span for batch write")

	// 检查是否已停止
	if w.store.isStopped() {
		w.logger.Warn().Msg("Store is stopping, writing directly")
		return w.writeSpanDirect(ctx, span)
	}

	// 非阻塞发送到批量写入缓冲区
	select {
	case w.store.spanBuffer <- span:
		return nil
	default:
		// 缓冲区满，直接写入
		w.logger.Warn().Msg("Span buffer full, writing directly")
		return w.writeSpanDirect(ctx, span)
	}
}

// writeSpanDirect 直接写入单个 span（fallback）
func (w *MySQLSpanWriter) writeSpanDirect(ctx context.Context, span *model.Span) error {
	tags := marshalTags(span.Tags)
	logs := marshalLogs(span.Logs)
	refs := marshalRefs(span.References)
	process := marshalProcess(span.Process)

	query := `
		INSERT INTO jaeger_spans (
			trace_id, span_id, operation_name, flags,
			start_time, duration, tags, logs, refs, process, service_name
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`

	_, err := w.db.ExecContext(ctx, query,
		span.TraceID.String(),
		span.SpanID.String(),
		span.OperationName,
		span.Flags,
		span.StartTime.UnixNano(),
		span.Duration.Nanoseconds(),
		tags,
		logs,
		refs,
		process,
		span.Process.ServiceName,
	)

	if err != nil {
		w.logger.Error().Err(err).Msg("Failed to write span directly")
		return err
	}

	return nil
}

// ====================
// SpanReader 实现
// ====================

type MySQLSpanReader struct {
	store  *MySQLStore
	db     *sql.DB
	logger zerolog.Logger
}

func (r *MySQLSpanReader) GetTrace(ctx context.Context, traceID model.TraceID) (*model.Trace, error) {
	r.logger.Debug().Str("trace_id", traceID.String()).Msg("Getting trace")

	query := `
		SELECT trace_id, span_id, operation_name, flags,
			   start_time, duration, tags, logs, refs, process, service_name
		FROM jaeger_spans
		WHERE trace_id = ?
		ORDER BY start_time ASC
	`

	rows, err := r.db.QueryContext(ctx, query, traceID.String())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	spans, err := scanSpans(rows)
	if err != nil {
		return nil, err
	}

	if len(spans) == 0 {
		return nil, spanstore.ErrTraceNotFound
	}

	return &model.Trace{Spans: spans}, nil
}

func (r *MySQLSpanReader) GetServices(ctx context.Context) ([]string, error) {
	r.logger.Debug().Msg("Getting services")

	// 检查缓存
	r.store.cacheMu.RLock()
	if r.store.servicesCache != nil && r.store.servicesCache.isValid() {
		services := r.store.servicesCache.data
		r.store.cacheMu.RUnlock()
		r.logger.Debug().Int("count", len(services)).Msg("Services from cache")
		return services, nil
	}
	r.store.cacheMu.RUnlock()

	// 查询数据库
	query := `SELECT service_name FROM jaeger_spans GROUP BY service_name`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var services []string
	for rows.Next() {
		var service string
		if err := rows.Scan(&service); err != nil {
			r.logger.Warn().Err(err).Msg("Failed to scan service")
			continue
		}
		services = append(services, service)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 更新缓存
	r.store.cacheMu.Lock()
	r.store.servicesCache = &cacheEntry[[]string]{
		data:      services,
		expiresAt: time.Now().Add(servicesCacheTTL),
	}
	r.store.cacheMu.Unlock()

	r.logger.Debug().Int("count", len(services)).Msg("Services from database")
	return services, nil
}

func (r *MySQLSpanReader) GetOperations(ctx context.Context, query spanstore.OperationQueryParameters) ([]spanstore.Operation, error) {
	r.logger.Debug().Str("service", query.ServiceName).Msg("Getting operations")

	// 检查缓存
	cacheKey := query.ServiceName
	r.store.cacheMu.RLock()
	if cache, ok := r.store.operationsCache[cacheKey]; ok && cache.isValid() {
		ops := cache.data
		r.store.cacheMu.RUnlock()
		r.logger.Debug().Int("count", len(ops)).Msg("Operations from cache")
		return ops, nil
	}
	r.store.cacheMu.RUnlock()

	// 查询数据库
	sqlQuery := `
		SELECT operation_name 
		FROM jaeger_spans 
		WHERE service_name = ?
		GROUP BY operation_name
	`

	rows, err := r.db.QueryContext(ctx, sqlQuery, query.ServiceName)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var operations []spanstore.Operation
	for rows.Next() {
		var opName string
		if err := rows.Scan(&opName); err != nil {
			r.logger.Warn().Err(err).Msg("Failed to scan operation")
			continue
		}
		operations = append(operations, spanstore.Operation{Name: opName})
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 更新缓存
	r.store.cacheMu.Lock()
	r.store.operationsCache[cacheKey] = &cacheEntry[[]spanstore.Operation]{
		data:      operations,
		expiresAt: time.Now().Add(operationsCacheTTL),
	}
	r.store.cacheMu.Unlock()

	r.logger.Debug().Int("count", len(operations)).Msg("Operations from database")
	return operations, nil
}

func (r *MySQLSpanReader) FindTraces(ctx context.Context, query *spanstore.TraceQueryParameters) ([]*model.Trace, error) {
	r.logger.Debug().Str("service", query.ServiceName).Msg("Finding traces")

	// Step 1: 获取符合条件的 trace IDs
	sqlQuery := `
		SELECT trace_id, MAX(start_time) as max_start_time
		FROM jaeger_spans
		WHERE service_name = ?
			AND start_time >= ?
			AND start_time <= ?
	`
	args := []interface{}{
		query.ServiceName,
		query.StartTimeMin.UnixNano(),
		query.StartTimeMax.UnixNano(),
	}

	if query.OperationName != "" {
		sqlQuery += " AND operation_name = ?"
		args = append(args, query.OperationName)
	}

	// 支持 Tags 过滤（全文搜索）
	if len(query.Tags) > 0 {
		for key, value := range query.Tags {
			// 使用 MATCH 进行全文搜索
			sqlQuery += " AND MATCH(?)"
			args = append(args, fmt.Sprintf("%s %s", key, value))
		}
	}

	// 支持 Duration 过滤
	if query.DurationMin > 0 {
		sqlQuery += " AND duration >= ?"
		args = append(args, query.DurationMin.Nanoseconds())
	}
	if query.DurationMax > 0 {
		sqlQuery += " AND duration <= ?"
		args = append(args, query.DurationMax.Nanoseconds())
	}

	sqlQuery += " GROUP BY trace_id ORDER BY max_start_time DESC LIMIT ?"
	args = append(args, query.NumTraces)

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}

	var traceIDs []string
	for rows.Next() {
		var traceIDStr string
		var maxStartTime int64
		if err := rows.Scan(&traceIDStr, &maxStartTime); err != nil {
			r.logger.Warn().Err(err).Msg("Failed to scan trace ID")
			continue
		}
		traceIDs = append(traceIDs, traceIDStr)
	}
	rows.Close()

	if err := rows.Err(); err != nil {
		return nil, err
	}

	if len(traceIDs) == 0 {
		return []*model.Trace{}, nil
	}

	// Step 2: 批量获取所有 spans（解决 N+1 问题）
	return r.getTracesByIDs(ctx, traceIDs)
}

// getTracesByIDs 批量获取多个 trace 的所有 spans
func (r *MySQLSpanReader) getTracesByIDs(ctx context.Context, traceIDs []string) ([]*model.Trace, error) {
	if len(traceIDs) == 0 {
		return []*model.Trace{}, nil
	}

	// 构建 IN 查询（ManticoreSearch 支持 IN）
	placeholders := make([]string, len(traceIDs))
	args := make([]interface{}, len(traceIDs))
	for i, id := range traceIDs {
		placeholders[i] = "?"
		args[i] = id
	}

	query := fmt.Sprintf(`
		SELECT trace_id, span_id, operation_name, flags,
			   start_time, duration, tags, logs, refs, process, service_name
		FROM jaeger_spans
		WHERE trace_id IN (%s)
		ORDER BY trace_id ASC, start_time ASC
	`, strings.Join(placeholders, ", "))

	rows, err := r.db.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("batch query failed: %w", err)
	}
	defer rows.Close()

	// 按 trace_id 分组
	traceMap := make(map[string][]*model.Span)
	for rows.Next() {
		span, err := scanSpan(rows)
		if err != nil {
			r.logger.Warn().Err(err).Msg("Failed to scan span")
			continue
		}
		traceIDStr := span.TraceID.String()
		traceMap[traceIDStr] = append(traceMap[traceIDStr], span)
	}

	if err := rows.Err(); err != nil {
		return nil, err
	}

	// 按原始顺序构建结果
	traces := make([]*model.Trace, 0, len(traceIDs))
	for _, traceIDStr := range traceIDs {
		if spans, ok := traceMap[traceIDStr]; ok && len(spans) > 0 {
			traces = append(traces, &model.Trace{Spans: spans})
		}
	}

	r.logger.Debug().
		Int("requested", len(traceIDs)).
		Int("found", len(traces)).
		Msg("Batch traces loaded")

	return traces, nil
}

func (r *MySQLSpanReader) FindTraceIDs(ctx context.Context, query *spanstore.TraceQueryParameters) ([]model.TraceID, error) {
	// 优化：直接返回 trace IDs，不加载完整 traces
	sqlQuery := `
		SELECT trace_id, MAX(start_time) as max_start_time
		FROM jaeger_spans
		WHERE service_name = ?
			AND start_time >= ?
			AND start_time <= ?
	`
	args := []interface{}{
		query.ServiceName,
		query.StartTimeMin.UnixNano(),
		query.StartTimeMax.UnixNano(),
	}

	if query.OperationName != "" {
		sqlQuery += " AND operation_name = ?"
		args = append(args, query.OperationName)
	}

	sqlQuery += " GROUP BY trace_id ORDER BY max_start_time DESC LIMIT ?"
	args = append(args, query.NumTraces)

	rows, err := r.db.QueryContext(ctx, sqlQuery, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var traceIDs []model.TraceID
	for rows.Next() {
		var traceIDStr string
		var maxStartTime int64
		if err := rows.Scan(&traceIDStr, &maxStartTime); err != nil {
			continue
		}
		if traceID, err := model.TraceIDFromString(traceIDStr); err == nil {
			traceIDs = append(traceIDs, traceID)
		}
	}

	return traceIDs, rows.Err()
}

// ====================
// DependencyReader 实现
// ====================

type MySQLDependencyReader struct {
	db     *sql.DB
	logger zerolog.Logger
}

func (r *MySQLDependencyReader) GetDependencies(ctx context.Context, endTs time.Time, lookback time.Duration) ([]model.DependencyLink, error) {
	r.logger.Debug().
		Time("end_time", endTs).
		Dur("lookback", lookback).
		Msg("Getting dependencies")

	startTs := endTs.Add(-lookback)

	// 查询所有 spans 的父子关系
	query := `
		SELECT trace_id, span_id, refs, service_name
		FROM jaeger_spans
		WHERE start_time >= ? AND start_time <= ?
	`

	rows, err := r.db.QueryContext(ctx, query, startTs.UnixNano(), endTs.UnixNano())
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	// 构建 span -> service 映射
	type spanInfo struct {
		traceID     string
		serviceName string
		parentSpans []string
	}
	spanMap := make(map[string]*spanInfo) // key: traceID:spanID

	for rows.Next() {
		var traceID, spanID, refsJSON, serviceName string
		if err := rows.Scan(&traceID, &spanID, &refsJSON, &serviceName); err != nil {
			continue
		}

		key := traceID + ":" + spanID
		info := &spanInfo{
			traceID:     traceID,
			serviceName: serviceName,
		}

		// 解析 refs 获取父 span
		var refs []model.SpanRef
		if err := json.Unmarshal([]byte(refsJSON), &refs); err == nil {
			for _, ref := range refs {
				if ref.RefType == model.SpanRefType_CHILD_OF {
					parentKey := traceID + ":" + ref.SpanID.String()
					info.parentSpans = append(info.parentSpans, parentKey)
				}
			}
		}

		spanMap[key] = info
	}

	// 统计依赖关系
	depCount := make(map[string]uint64) // key: parent_service -> child_service

	for _, info := range spanMap {
		for _, parentKey := range info.parentSpans {
			if parentInfo, ok := spanMap[parentKey]; ok {
				if parentInfo.serviceName != info.serviceName {
					depKey := parentInfo.serviceName + " -> " + info.serviceName
					depCount[depKey]++
				}
			}
		}
	}

	// 转换为 DependencyLink
	var deps []model.DependencyLink
	for key, count := range depCount {
		parts := strings.Split(key, " -> ")
		if len(parts) == 2 {
			deps = append(deps, model.DependencyLink{
				Parent:    parts[0],
				Child:     parts[1],
				CallCount: count,
			})
		}
	}

	r.logger.Debug().Int("count", len(deps)).Msg("Dependencies calculated")
	return deps, nil
}

// ====================
// 辅助函数
// ====================

// scanSpans 批量扫描 spans（预分配容量）
func scanSpans(rows *sql.Rows) ([]*model.Span, error) {
	spans := make([]*model.Span, 0, 64) // 预分配常见大小
	for rows.Next() {
		span, err := scanSpan(rows)
		if err != nil {
			continue
		}
		spans = append(spans, span)
	}
	return spans, rows.Err()
}

func scanSpan(rows *sql.Rows) (*model.Span, error) {
	var (
		traceIDStr  string
		spanIDStr   string
		opName      string
		flags       int
		startTime   int64
		duration    int64
		tagsJSON    string
		logsJSON    string
		refsJSON    string
		processJSON string
		serviceName string
	)

	err := rows.Scan(
		&traceIDStr, &spanIDStr, &opName, &flags,
		&startTime, &duration, &tagsJSON, &logsJSON,
		&refsJSON, &processJSON, &serviceName,
	)
	if err != nil {
		return nil, err
	}

	traceID, _ := model.TraceIDFromString(traceIDStr)
	spanID, _ := model.SpanIDFromString(spanIDStr)

	span := &model.Span{
		TraceID:       traceID,
		SpanID:        spanID,
		OperationName: opName,
		Flags:         model.Flags(flags),
		StartTime:     time.Unix(0, startTime),
		Duration:      time.Duration(duration),
	}

	// 反序列化 JSON 字段
	json.Unmarshal([]byte(tagsJSON), &span.Tags)
	json.Unmarshal([]byte(logsJSON), &span.Logs)
	json.Unmarshal([]byte(refsJSON), &span.References)
	json.Unmarshal([]byte(processJSON), &span.Process)

	if span.Process == nil {
		span.Process = &model.Process{
			ServiceName: serviceName,
		}
	}

	return span, nil
}
