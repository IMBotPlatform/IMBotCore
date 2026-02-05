// Package scheduler 提供定时任务调度功能。
// 支持 Cron 表达式、固定间隔、一次性任务三种调度方式。
package scheduler

import (
	"context"
	"time"
)

// ScheduleType 调度类型
type ScheduleType string

const (
	// ScheduleTypeCron Cron 表达式调度（如 "0 9 * * 1-5"）
	ScheduleTypeCron ScheduleType = "cron"
	// ScheduleTypeInterval 固定间隔调度（毫秒）
	ScheduleTypeInterval ScheduleType = "interval"
	// ScheduleTypeOnce 一次性任务（ISO 8601 时间）
	ScheduleTypeOnce ScheduleType = "once"
)

// TaskStatus 任务状态
type TaskStatus string

const (
	// TaskStatusActive 任务激活中
	TaskStatusActive TaskStatus = "active"
	// TaskStatusPaused 任务已暂停
	TaskStatusPaused TaskStatus = "paused"
	// TaskStatusCompleted 任务已完成（一次性任务或达到最大执行次数）
	TaskStatusCompleted TaskStatus = "completed"
	// TaskStatusFailed 任务失败
	TaskStatusFailed TaskStatus = "failed"
)

// ContextMode 上下文模式
type ContextMode string

const (
	// ContextModeIsolated 每次执行使用独立会话
	ContextModeIsolated ContextMode = "isolated"
	// ContextModeGroup 复用群组会话
	ContextModeGroup ContextMode = "group"
)

// Task 定时任务
type Task struct {
	ID            string            `json:"id"`             // 任务唯一标识
	GroupID       string            `json:"group_id"`       // 关联的群组 ID
	ChatID        string            `json:"chat_id"`        // 消息发送目标
	Platform      string            `json:"platform"`       // 平台标识
	Prompt        string            `json:"prompt"`         // 执行提示词
	ScheduleType  ScheduleType      `json:"schedule_type"`  // 调度类型
	ScheduleValue string            `json:"schedule_value"` // 调度值
	ContextMode   ContextMode       `json:"context_mode"`   // 上下文模式
	Status        TaskStatus        `json:"status"`         // 任务状态
	NextRun       *time.Time        `json:"next_run"`       // 下次执行时间
	LastRun       *time.Time        `json:"last_run"`       // 上次执行时间
	LastResult    string            `json:"last_result"`    // 上次执行结果
	RunCount      int               `json:"run_count"`      // 执行次数
	MaxRuns       int               `json:"max_runs"`       // 最大执行次数（0 = 无限）
	Metadata      map[string]string `json:"metadata"`       // 扩展元数据
	CreatedAt     time.Time         `json:"created_at"`     // 创建时间
	UpdatedAt     time.Time         `json:"updated_at"`     // 更新时间
}

// TaskRunLog 任务执行日志
type TaskRunLog struct {
	ID         int64     `json:"id"`          // 日志 ID
	TaskID     string    `json:"task_id"`     // 任务 ID
	RunAt      time.Time `json:"run_at"`      // 执行时间
	DurationMs int64     `json:"duration_ms"` // 执行耗时（毫秒）
	Status     string    `json:"status"`      // success / error
	Result     string    `json:"result"`      // 执行结果
	Error      string    `json:"error"`       // 错误信息
}

// CreateTaskRequest 创建任务请求
type CreateTaskRequest struct {
	GroupID       string            // 群组 ID
	ChatID        string            // 会话 ID
	Platform      string            // 平台标识
	Prompt        string            // 执行提示词
	ScheduleType  ScheduleType      // 调度类型
	ScheduleValue string            // 调度值
	ContextMode   ContextMode       // 上下文模式（默认 isolated）
	MaxRuns       int               // 最大执行次数（0 = 无限）
	Metadata      map[string]string // 扩展元数据
}

// Scheduler 定时任务调度器接口
type Scheduler interface {
	// Create 创建新任务
	// 参数：ctx - 上下文，req - 创建请求
	// 返回：创建的任务和可能的错误
	Create(ctx context.Context, req CreateTaskRequest) (*Task, error)

	// Get 获取任务详情
	// 参数：ctx - 上下文，taskID - 任务 ID
	// 返回：任务详情和可能的错误
	Get(ctx context.Context, taskID string) (*Task, error)

	// Update 更新任务
	// 参数：ctx - 上下文，taskID - 任务 ID，updates - 更新字段
	// 返回：可能的错误
	Update(ctx context.Context, taskID string, updates map[string]interface{}) error

	// Delete 删除任务
	// 参数：ctx - 上下文，taskID - 任务 ID
	// 返回：可能的错误
	Delete(ctx context.Context, taskID string) error

	// Pause 暂停任务
	// 参数：ctx - 上下文，taskID - 任务 ID
	// 返回：可能的错误
	Pause(ctx context.Context, taskID string) error

	// Resume 恢复任务
	// 参数：ctx - 上下文，taskID - 任务 ID
	// 返回：可能的错误
	Resume(ctx context.Context, taskID string) error

	// ListByGroup 列出群组的所有任务
	// 参数：ctx - 上下文，groupID - 群组 ID
	// 返回：任务列表和可能的错误
	ListByGroup(ctx context.Context, groupID string) ([]Task, error)

	// ListAll 列出所有任务
	// 参数：ctx - 上下文
	// 返回：任务列表和可能的错误
	ListAll(ctx context.Context) ([]Task, error)

	// GetDueTasks 获取到期任务
	// 参数：ctx - 上下文
	// 返回：到期任务列表和可能的错误
	GetDueTasks(ctx context.Context) ([]Task, error)

	// LogRun 记录执行日志
	// 参数：ctx - 上下文，log - 执行日志
	// 返回：可能的错误
	LogRun(ctx context.Context, log TaskRunLog) error

	// GetRunLogs 获取执行日志
	// 参数：ctx - 上下文，taskID - 任务 ID，limit - 返回条数
	// 返回：执行日志列表和可能的错误
	GetRunLogs(ctx context.Context, taskID string, limit int) ([]TaskRunLog, error)

	// UpdateAfterRun 执行后更新任务状态
	// 参数：ctx - 上下文，taskID - 任务 ID，result - 执行结果，err - 执行错误
	// 返回：可能的错误
	UpdateAfterRun(ctx context.Context, taskID string, result string, err error) error

	// OnDue 注册到期任务回调
	// 参数：handler - 任务处理函数
	OnDue(handler TaskHandler)

	// Start 启动调度循环
	// 参数：ctx - 上下文
	// 返回：可能的错误
	Start(ctx context.Context) error

	// Stop 停止调度器
	// 返回：可能的错误
	Stop() error
}

// TaskHandler 任务处理函数
type TaskHandler func(ctx context.Context, task Task) error

// Config 调度器配置
type Config struct {
	// DBPath SQLite 数据库路径
	DBPath string
	// PollInterval 轮询间隔，默认 60s
	PollInterval time.Duration
	// Timezone 时区，默认系统时区
	Timezone string
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		DBPath:       "scheduler.db",
		PollInterval: 60 * time.Second,
		Timezone:     "",
	}
}
