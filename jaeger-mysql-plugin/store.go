package main

import (
	"context"
	"database/sql"
	"encoding/json"
	"time"

	"github.com/jaegertracing/jaeger/model"
	"github.com/jaegertracing/jaeger/storage/dependencystore"
	"github.com/jaegertracing/jaeger/storage/spanstore"
	"github.com/rs/zerolog"
)

type MySQLStore struct {
	db     *sql.DB
	logger zerolog.Logger
}

func NewMySQLStore(db *sql.DB, logger zerolog.Logger) *MySQLStore {
	return &MySQLStore{
		db:     db,
		logger: logger,
	}
}

// 实现 StoragePluginServer 接口
func (s *MySQLStore) SpanReader() spanstore.Reader {
	return &MySQLSpanReader{
		db:     s.db,
		logger: s.logger,
	}
}

func (s *MySQLStore) SpanWriter() spanstore.Writer {
	return &MySQLSpanWriter{
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
	db     *sql.DB
	logger zerolog.Logger
}

func (w *MySQLSpanWriter) WriteSpan(ctx context.Context, span *model.Span) error {
	w.logger.Debug().Str("trace_id", span.TraceID.String()).Str("span_id", span.SpanID.String()).Msg("Writing span")

	// 序列化复杂字段
	tags, _ := json.Marshal(span.Tags)
	logs, _ := json.Marshal(span.Logs)
	refs, _ := json.Marshal(span.References)
	process, _ := json.Marshal(span.Process)

	// ManticoreSearch RT 表使用 INSERT
	// 注意：需要在 DSN 中添加 interpolateParams=true 来避免预处理语句
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
		string(tags),
		string(logs),
		string(refs),
		string(process),
		span.Process.ServiceName,
	)

	if err != nil {
		w.logger.Error().Err(err).Str("query", "REPLACE INTO jaeger_spans").Msg("Failed to write span")
		return err
	}

	w.logger.Info().Str("trace_id", span.TraceID.String()).Str("service", span.Process.ServiceName).Msg("Successfully wrote span")
	return nil
}

// ====================
// SpanReader 实现
// ====================

type MySQLSpanReader struct {
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
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			r.logger.Warn().Err(closeErr).Msg("Failed to close rows in GetTrace")
		}
	}()

	var spans []*model.Span
	for rows.Next() {
		span, err := scanSpan(rows)
		if err != nil {
			r.logger.Error().Err(err).Msg("Failed to scan span")
			continue
		}
		spans = append(spans, span)
	}

	if len(spans) == 0 {
		return nil, spanstore.ErrTraceNotFound
	}

	return &model.Trace{
		Spans: spans,
	}, nil
}

func (r *MySQLSpanReader) GetServices(ctx context.Context) ([]string, error) {
	r.logger.Debug().Msg("Getting services")

	// ManticoreSearch 的 GROUP BY 不能直接用 ORDER BY，移除排序
	query := `SELECT service_name FROM jaeger_spans GROUP BY service_name`

	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, err
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			r.logger.Warn().Err(closeErr).Msg("Failed to close rows in GetServices")
		}
	}()

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
		r.logger.Error().Err(err).Msg("Error iterating rows in GetServices")
		return nil, err
	}

	return services, nil
}

func (r *MySQLSpanReader) GetOperations(ctx context.Context, query spanstore.OperationQueryParameters) ([]spanstore.Operation, error) {
	r.logger.Debug().Str("service", query.ServiceName).Msg("Getting operations")

	// ManticoreSearch 不支持 DISTINCT，使用 GROUP BY（移除 ORDER BY）
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
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			r.logger.Warn().Err(closeErr).Msg("Failed to close rows in GetOperations")
		}
	}()

	var operations []spanstore.Operation
	for rows.Next() {
		var opName string
		if err := rows.Scan(&opName); err != nil {
			r.logger.Warn().Err(err).Msg("Failed to scan operation")
			continue
		}
		operations = append(operations, spanstore.Operation{
			Name: opName,
		})
	}

	if err := rows.Err(); err != nil {
		r.logger.Error().Err(err).Msg("Error iterating rows in GetOperations")
		return nil, err
	}

	return operations, nil
}

func (r *MySQLSpanReader) FindTraces(ctx context.Context, query *spanstore.TraceQueryParameters) ([]*model.Trace, error) {
	r.logger.Debug().Str("service", query.ServiceName).Msg("Finding traces")

	// 构建查询
	// ManticoreSearch GROUP BY 需要使用聚合函数才能排序
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
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			r.logger.Warn().Err(closeErr).Msg("Failed to close rows")
		}
	}()

	var traces []*model.Trace
	var traceIDs []string

	// 先收集所有 trace IDs，避免在循环中嵌套查询
	for rows.Next() {
		var traceIDStr string
		var maxStartTime int64 // MAX(start_time) 结果，我们不使用但必须扫描
		if err := rows.Scan(&traceIDStr, &maxStartTime); err != nil {
			r.logger.Warn().Err(err).Msg("Failed to scan trace ID")
			continue
		}
		traceIDs = append(traceIDs, traceIDStr)
	}

	// 检查是否有行扫描错误
	if err := rows.Err(); err != nil {
		r.logger.Error().Err(err).Msg("Error iterating rows")
		return nil, err
	}

	// 批量获取 traces
	for _, traceIDStr := range traceIDs {
		traceID, err := model.TraceIDFromString(traceIDStr)
		if err != nil {
			r.logger.Warn().Err(err).Str("trace_id", traceIDStr).Msg("Failed to parse trace ID")
			continue
		}

		trace, err := r.GetTrace(ctx, traceID)
		if err != nil {
			r.logger.Warn().Err(err).Str("trace_id", traceIDStr).Msg("Failed to get trace")
			continue
		}

		traces = append(traces, trace)
	}

	return traces, nil
}

func (r *MySQLSpanReader) FindTraceIDs(ctx context.Context, query *spanstore.TraceQueryParameters) ([]model.TraceID, error) {
	traces, err := r.FindTraces(ctx, query)
	if err != nil {
		return nil, err
	}

	var traceIDs []model.TraceID
	for _, trace := range traces {
		if len(trace.Spans) > 0 {
			traceIDs = append(traceIDs, trace.Spans[0].TraceID)
		}
	}

	return traceIDs, nil
}

// ====================
// DependencyReader 实现
// ====================

type MySQLDependencyReader struct {
	db     *sql.DB
	logger zerolog.Logger
}

func (r *MySQLDependencyReader) GetDependencies(ctx context.Context, endTs time.Time, lookback time.Duration) ([]model.DependencyLink, error) {
	// 简化实现：返回空依赖
	return []model.DependencyLink{}, nil
}

// ====================
// 辅助函数
// ====================

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
