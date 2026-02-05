package scheduler

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/robfig/cron/v3"
	_ "modernc.org/sqlite"
)

// SQLiteScheduler 基于 SQLite 的调度器实现
type SQLiteScheduler struct {
	db           *sql.DB
	pollInterval time.Duration
	timezone     *time.Location
	handler      TaskHandler
	stopCh       chan struct{}
	wg           sync.WaitGroup
	mu           sync.RWMutex
	started      bool
}

// New 创建 SQLite 调度器实例
// 参数：cfg - 调度器配置
// 返回：Scheduler 实现和可能的错误
func New(cfg Config) (Scheduler, error) {
	dbPath := cfg.DBPath
	if dbPath == "" {
		dbPath = DefaultConfig().DBPath
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("open database: %w", err)
	}

	pollInterval := cfg.PollInterval
	if pollInterval == 0 {
		pollInterval = DefaultConfig().PollInterval
	}

	tz := time.Local
	if cfg.Timezone != "" {
		if loc, err := time.LoadLocation(cfg.Timezone); err == nil {
			tz = loc
		}
	}

	s := &SQLiteScheduler{
		db:           db,
		pollInterval: pollInterval,
		timezone:     tz,
		stopCh:       make(chan struct{}),
	}

	if err := s.createTables(); err != nil {
		db.Close()
		return nil, fmt.Errorf("create tables: %w", err)
	}

	return s, nil
}

func (s *SQLiteScheduler) createTables() error {
	_, err := s.db.Exec(`
		CREATE TABLE IF NOT EXISTS scheduled_tasks (
			id TEXT PRIMARY KEY,
			group_id TEXT NOT NULL,
			chat_id TEXT NOT NULL,
			platform TEXT NOT NULL DEFAULT 'default',
			prompt TEXT NOT NULL,
			schedule_type TEXT NOT NULL CHECK (schedule_type IN ('cron', 'interval', 'once')),
			schedule_value TEXT NOT NULL,
			context_mode TEXT DEFAULT 'isolated' CHECK (context_mode IN ('isolated', 'group')),
			status TEXT DEFAULT 'active' CHECK (status IN ('active', 'paused', 'completed', 'failed')),
			next_run TEXT,
			last_run TEXT,
			last_result TEXT,
			run_count INTEGER DEFAULT 0,
			max_runs INTEGER DEFAULT 0,
			metadata TEXT,
			created_at TEXT NOT NULL,
			updated_at TEXT NOT NULL
		);
		CREATE INDEX IF NOT EXISTS idx_tasks_group ON scheduled_tasks(group_id);
		CREATE INDEX IF NOT EXISTS idx_tasks_status ON scheduled_tasks(status);
		CREATE INDEX IF NOT EXISTS idx_tasks_next_run ON scheduled_tasks(next_run);

		CREATE TABLE IF NOT EXISTS task_run_logs (
			id INTEGER PRIMARY KEY AUTOINCREMENT,
			task_id TEXT NOT NULL,
			run_at TEXT NOT NULL,
			duration_ms INTEGER NOT NULL,
			status TEXT NOT NULL CHECK (status IN ('success', 'error')),
			result TEXT,
			error TEXT,
			FOREIGN KEY (task_id) REFERENCES scheduled_tasks(id) ON DELETE CASCADE
		);
		CREATE INDEX IF NOT EXISTS idx_logs_task ON task_run_logs(task_id, run_at DESC);
	`)
	return err
}

// Create 创建新任务
func (s *SQLiteScheduler) Create(ctx context.Context, req CreateTaskRequest) (*Task, error) {
	now := time.Now()

	contextMode := req.ContextMode
	if contextMode == "" {
		contextMode = ContextModeIsolated
	}

	task := &Task{
		ID:            uuid.New().String(),
		GroupID:       req.GroupID,
		ChatID:        req.ChatID,
		Platform:      req.Platform,
		Prompt:        req.Prompt,
		ScheduleType:  req.ScheduleType,
		ScheduleValue: req.ScheduleValue,
		ContextMode:   contextMode,
		Status:        TaskStatusActive,
		MaxRuns:       req.MaxRuns,
		Metadata:      req.Metadata,
		CreatedAt:     now,
		UpdatedAt:     now,
	}

	// 计算下次执行时间
	nextRun, err := s.calculateNextRun(task.ScheduleType, task.ScheduleValue, nil)
	if err != nil {
		return nil, fmt.Errorf("invalid schedule: %w", err)
	}
	task.NextRun = nextRun

	// 序列化 metadata
	metadataJSON := "{}"
	if req.Metadata != nil {
		if data, err := json.Marshal(req.Metadata); err == nil {
			metadataJSON = string(data)
		}
	}

	// 插入数据库
	_, err = s.db.ExecContext(ctx, `
		INSERT INTO scheduled_tasks 
		(id, group_id, chat_id, platform, prompt, schedule_type, schedule_value, 
		 context_mode, status, next_run, max_runs, metadata, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`, task.ID, task.GroupID, task.ChatID, task.Platform, task.Prompt,
		task.ScheduleType, task.ScheduleValue, task.ContextMode, task.Status,
		formatTime(task.NextRun), task.MaxRuns, metadataJSON,
		formatTime(&task.CreatedAt), formatTime(&task.UpdatedAt))

	if err != nil {
		return nil, fmt.Errorf("insert task: %w", err)
	}

	return task, nil
}

// calculateNextRun 计算下次执行时间
func (s *SQLiteScheduler) calculateNextRun(schedType ScheduleType, schedValue string, lastRun *time.Time) (*time.Time, error) {
	now := time.Now().In(s.timezone)

	switch schedType {
	case ScheduleTypeCron:
		parser := cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow)
		schedule, err := parser.Parse(schedValue)
		if err != nil {
			return nil, fmt.Errorf("parse cron: %w", err)
		}
		next := schedule.Next(now)
		return &next, nil

	case ScheduleTypeInterval:
		ms, err := strconv.ParseInt(schedValue, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("parse interval: %w", err)
		}
		next := now.Add(time.Duration(ms) * time.Millisecond)
		return &next, nil

	case ScheduleTypeOnce:
		t, err := time.Parse(time.RFC3339, schedValue)
		if err != nil {
			return nil, fmt.Errorf("parse once time: %w", err)
		}
		if t.Before(now) {
			return nil, nil // 已过期
		}
		return &t, nil

	default:
		return nil, fmt.Errorf("unknown schedule type: %s", schedType)
	}
}

// Get 获取任务详情
func (s *SQLiteScheduler) Get(ctx context.Context, taskID string) (*Task, error) {
	row := s.db.QueryRowContext(ctx, `
		SELECT id, group_id, chat_id, platform, prompt, schedule_type, schedule_value,
		       context_mode, status, next_run, last_run, last_result, run_count, max_runs,
		       metadata, created_at, updated_at
		FROM scheduled_tasks WHERE id = ?
	`, taskID)

	return s.scanTask(row)
}

func (s *SQLiteScheduler) scanTask(row *sql.Row) (*Task, error) {
	var task Task
	var nextRun, lastRun, createdAt, updatedAt, metadataJSON sql.NullString
	var lastResult sql.NullString

	err := row.Scan(
		&task.ID, &task.GroupID, &task.ChatID, &task.Platform, &task.Prompt,
		&task.ScheduleType, &task.ScheduleValue, &task.ContextMode, &task.Status,
		&nextRun, &lastRun, &lastResult, &task.RunCount, &task.MaxRuns,
		&metadataJSON, &createdAt, &updatedAt,
	)
	if err != nil {
		if err == sql.ErrNoRows {
			return nil, fmt.Errorf("task not found")
		}
		return nil, err
	}

	task.NextRun = parseTime(nextRun.String)
	task.LastRun = parseTime(lastRun.String)
	task.LastResult = lastResult.String
	task.CreatedAt = *parseTime(createdAt.String)
	task.UpdatedAt = *parseTime(updatedAt.String)

	if metadataJSON.Valid && metadataJSON.String != "" {
		json.Unmarshal([]byte(metadataJSON.String), &task.Metadata)
	}

	return &task, nil
}

// Update 更新任务
func (s *SQLiteScheduler) Update(ctx context.Context, taskID string, updates map[string]interface{}) error {
	// 简化实现：只支持常用字段更新
	if prompt, ok := updates["prompt"].(string); ok {
		_, err := s.db.ExecContext(ctx, `
			UPDATE scheduled_tasks SET prompt = ?, updated_at = ? WHERE id = ?
		`, prompt, formatTime(ptr(time.Now())), taskID)
		return err
	}
	return nil
}

// Delete 删除任务
func (s *SQLiteScheduler) Delete(ctx context.Context, taskID string) error {
	_, err := s.db.ExecContext(ctx, `DELETE FROM scheduled_tasks WHERE id = ?`, taskID)
	return err
}

// Pause 暂停任务
func (s *SQLiteScheduler) Pause(ctx context.Context, taskID string) error {
	_, err := s.db.ExecContext(ctx, `
		UPDATE scheduled_tasks SET status = ?, updated_at = ? WHERE id = ?
	`, TaskStatusPaused, formatTime(ptr(time.Now())), taskID)
	return err
}

// Resume 恢复任务
func (s *SQLiteScheduler) Resume(ctx context.Context, taskID string) error {
	task, err := s.Get(ctx, taskID)
	if err != nil {
		return err
	}

	// 重新计算下次执行时间
	nextRun, err := s.calculateNextRun(task.ScheduleType, task.ScheduleValue, task.LastRun)
	if err != nil {
		return err
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE scheduled_tasks SET status = ?, next_run = ?, updated_at = ? WHERE id = ?
	`, TaskStatusActive, formatTime(nextRun), formatTime(ptr(time.Now())), taskID)
	return err
}

// ListByGroup 列出群组的所有任务
func (s *SQLiteScheduler) ListByGroup(ctx context.Context, groupID string) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, group_id, chat_id, platform, prompt, schedule_type, schedule_value,
		       context_mode, status, next_run, last_run, last_result, run_count, max_runs,
		       metadata, created_at, updated_at
		FROM scheduled_tasks WHERE group_id = ? ORDER BY created_at DESC
	`, groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanTasks(rows)
}

// ListAll 列出所有任务
func (s *SQLiteScheduler) ListAll(ctx context.Context) ([]Task, error) {
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, group_id, chat_id, platform, prompt, schedule_type, schedule_value,
		       context_mode, status, next_run, last_run, last_result, run_count, max_runs,
		       metadata, created_at, updated_at
		FROM scheduled_tasks ORDER BY created_at DESC
	`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanTasks(rows)
}

func (s *SQLiteScheduler) scanTasks(rows *sql.Rows) ([]Task, error) {
	var tasks []Task
	for rows.Next() {
		var task Task
		var nextRun, lastRun, createdAt, updatedAt, metadataJSON sql.NullString
		var lastResult sql.NullString

		err := rows.Scan(
			&task.ID, &task.GroupID, &task.ChatID, &task.Platform, &task.Prompt,
			&task.ScheduleType, &task.ScheduleValue, &task.ContextMode, &task.Status,
			&nextRun, &lastRun, &lastResult, &task.RunCount, &task.MaxRuns,
			&metadataJSON, &createdAt, &updatedAt,
		)
		if err != nil {
			return nil, err
		}

		task.NextRun = parseTime(nextRun.String)
		task.LastRun = parseTime(lastRun.String)
		task.LastResult = lastResult.String
		task.CreatedAt = *parseTime(createdAt.String)
		task.UpdatedAt = *parseTime(updatedAt.String)

		if metadataJSON.Valid && metadataJSON.String != "" {
			json.Unmarshal([]byte(metadataJSON.String), &task.Metadata)
		}

		tasks = append(tasks, task)
	}
	return tasks, rows.Err()
}

// GetDueTasks 获取到期任务
func (s *SQLiteScheduler) GetDueTasks(ctx context.Context) ([]Task, error) {
	now := formatTime(ptr(time.Now()))
	rows, err := s.db.QueryContext(ctx, `
		SELECT id, group_id, chat_id, platform, prompt, schedule_type, schedule_value,
		       context_mode, status, next_run, last_run, last_result, run_count, max_runs,
		       metadata, created_at, updated_at
		FROM scheduled_tasks 
		WHERE status = ? AND next_run IS NOT NULL AND next_run <= ?
		ORDER BY next_run ASC
	`, TaskStatusActive, now)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	return s.scanTasks(rows)
}

// LogRun 记录执行日志
func (s *SQLiteScheduler) LogRun(ctx context.Context, log TaskRunLog) error {
	_, err := s.db.ExecContext(ctx, `
		INSERT INTO task_run_logs (task_id, run_at, duration_ms, status, result, error)
		VALUES (?, ?, ?, ?, ?, ?)
	`, log.TaskID, formatTime(&log.RunAt), log.DurationMs, log.Status, log.Result, log.Error)
	return err
}

// GetRunLogs 获取执行日志
func (s *SQLiteScheduler) GetRunLogs(ctx context.Context, taskID string, limit int) ([]TaskRunLog, error) {
	if limit <= 0 {
		limit = 100
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, task_id, run_at, duration_ms, status, result, error
		FROM task_run_logs WHERE task_id = ?
		ORDER BY run_at DESC LIMIT ?
	`, taskID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var logs []TaskRunLog
	for rows.Next() {
		var log TaskRunLog
		var runAt string
		var result, errStr sql.NullString

		err := rows.Scan(&log.ID, &log.TaskID, &runAt, &log.DurationMs, &log.Status, &result, &errStr)
		if err != nil {
			return nil, err
		}

		log.RunAt = *parseTime(runAt)
		log.Result = result.String
		log.Error = errStr.String
		logs = append(logs, log)
	}
	return logs, rows.Err()
}

// UpdateAfterRun 执行后更新任务状态
func (s *SQLiteScheduler) UpdateAfterRun(ctx context.Context, taskID string, result string, runErr error) error {
	task, err := s.Get(ctx, taskID)
	if err != nil {
		return err
	}

	now := time.Now()
	task.RunCount++
	task.LastRun = &now
	task.LastResult = result
	task.UpdatedAt = now

	// 判断是否完成
	if task.ScheduleType == ScheduleTypeOnce {
		task.Status = TaskStatusCompleted
		task.NextRun = nil
	} else if task.MaxRuns > 0 && task.RunCount >= task.MaxRuns {
		task.Status = TaskStatusCompleted
		task.NextRun = nil
	} else {
		// 计算下次执行时间
		nextRun, _ := s.calculateNextRun(task.ScheduleType, task.ScheduleValue, task.LastRun)
		task.NextRun = nextRun
	}

	if runErr != nil {
		task.Status = TaskStatusFailed
	}

	_, err = s.db.ExecContext(ctx, `
		UPDATE scheduled_tasks 
		SET status = ?, next_run = ?, last_run = ?, last_result = ?, 
		    run_count = ?, updated_at = ?
		WHERE id = ?
	`, task.Status, formatTime(task.NextRun), formatTime(task.LastRun),
		task.LastResult, task.RunCount, formatTime(&task.UpdatedAt), taskID)

	return err
}

// OnDue 注册到期任务回调
func (s *SQLiteScheduler) OnDue(handler TaskHandler) {
	s.mu.Lock()
	s.handler = handler
	s.mu.Unlock()
}

// Start 启动调度循环
func (s *SQLiteScheduler) Start(ctx context.Context) error {
	s.mu.Lock()
	if s.started {
		s.mu.Unlock()
		return nil
	}
	s.started = true
	s.mu.Unlock()

	s.wg.Add(1)
	go func() {
		defer s.wg.Done()
		ticker := time.NewTicker(s.pollInterval)
		defer ticker.Stop()

		// 启动时立即检查一次
		s.processDueTasks(ctx)

		for {
			select {
			case <-ctx.Done():
				return
			case <-s.stopCh:
				return
			case <-ticker.C:
				s.processDueTasks(ctx)
			}
		}
	}()
	return nil
}

func (s *SQLiteScheduler) processDueTasks(ctx context.Context) {
	tasks, err := s.GetDueTasks(ctx)
	if err != nil {
		return
	}

	s.mu.RLock()
	handler := s.handler
	s.mu.RUnlock()

	if handler == nil {
		return
	}

	for _, task := range tasks {
		select {
		case <-ctx.Done():
			return
		default:
		}

		start := time.Now()
		err := handler(ctx, task)
		duration := time.Since(start)

		// 记录日志
		status := "success"
		errStr := ""
		if err != nil {
			status = "error"
			errStr = err.Error()
		}

		s.LogRun(ctx, TaskRunLog{
			TaskID:     task.ID,
			RunAt:      start,
			DurationMs: duration.Milliseconds(),
			Status:     status,
			Result:     task.LastResult,
			Error:      errStr,
		})

		// 更新任务状态
		s.UpdateAfterRun(ctx, task.ID, "", err)
	}
}

// Stop 停止调度器
func (s *SQLiteScheduler) Stop() error {
	s.mu.Lock()
	if !s.started {
		s.mu.Unlock()
		return nil
	}
	s.mu.Unlock()

	close(s.stopCh)
	s.wg.Wait()
	return s.db.Close()
}

// formatTime 格式化时间为 RFC3339
func formatTime(t *time.Time) string {
	if t == nil {
		return ""
	}
	return t.Format(time.RFC3339)
}

// parseTime 解析 RFC3339 时间
func parseTime(s string) *time.Time {
	if s == "" {
		return nil
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return nil
	}
	return &t
}

// ptr 返回时间指针
func ptr(t time.Time) *time.Time {
	return &t
}
