// Package callback 提供 AI Agent 回调接口。
// 允许 AI Agent 在执行过程中与主程序交互，如发送消息、创建任务等。
package callback

import (
	"context"
	"time"

	"github.com/IMBotPlatform/IMBotCore/pkg/scheduler"
)

// Callback AI 回调接口
// 提供 Agent 执行过程中与主程序交互的能力。
type Callback interface {
	// SendMessage 发送消息到指定会话
	// 参数：ctx - 上下文，req - 发送请求
	// 返回：可能的错误
	SendMessage(ctx context.Context, req SendMessageRequest) error

	// ScheduleTask 创建定时任务
	// 参数：ctx - 上下文，req - 任务创建请求
	// 返回：创建的任务和可能的错误
	ScheduleTask(ctx context.Context, req scheduler.CreateTaskRequest) (*scheduler.Task, error)

	// PauseTask 暂停任务
	// 参数：ctx - 上下文，taskID - 任务 ID
	// 返回：可能的错误
	PauseTask(ctx context.Context, taskID string) error

	// ResumeTask 恢复任务
	// 参数：ctx - 上下文，taskID - 任务 ID
	// 返回：可能的错误
	ResumeTask(ctx context.Context, taskID string) error

	// CancelTask 取消任务
	// 参数：ctx - 上下文，taskID - 任务 ID
	// 返回：可能的错误
	CancelTask(ctx context.Context, taskID string) error

	// ListTasks 列出群组的任务
	// 参数：ctx - 上下文，groupID - 群组 ID
	// 返回：任务列表和可能的错误
	ListTasks(ctx context.Context, groupID string) ([]scheduler.Task, error)

	// RequestApproval 请求人工审批（阻塞等待）
	// 参数：ctx - 上下文，req - 审批请求
	// 返回：审批响应和可能的错误
	RequestApproval(ctx context.Context, req ApprovalRequest) (*ApprovalResponse, error)
}

// SendMessageRequest 发送消息请求
type SendMessageRequest struct {
	ChatID   string // 目标会话 ID
	Platform string // 平台标识
	Text     string // 消息内容
	ReplyTo  string // 回复的消息 ID（可选）
}

// ApprovalRequest 审批请求
type ApprovalRequest struct {
	ChatID      string        // 发送审批请求的会话
	Title       string        // 审批标题
	Description string        // 审批描述
	Timeout     time.Duration // 超时时间
}

// ApprovalResponse 审批响应
type ApprovalResponse struct {
	Approved   bool   // 是否批准
	ApprovedBy string // 批准人
	Comment    string // 批注
}

// MessageType IPC 消息类型
type MessageType string

const (
	// MessageTypeSend 发送消息
	MessageTypeSend MessageType = "message"
	// MessageTypeApprovalRequest 审批请求
	MessageTypeApprovalRequest MessageType = "approval_request"
	// MessageTypeApprovalResponse 审批响应
	MessageTypeApprovalResponse MessageType = "approval_response"
)

// IPCMessage IPC 消息结构（用于文件系统 IPC）
type IPCMessage struct {
	Type     MessageType `json:"type"`
	ID       string      `json:"id,omitempty"`
	ChatID   string      `json:"chat_id,omitempty"`
	Platform string      `json:"platform,omitempty"`
	Text     string      `json:"text,omitempty"`
	ReplyTo  string      `json:"reply_to,omitempty"`

	// 审批相关
	Title       string `json:"title,omitempty"`
	Description string `json:"description,omitempty"`
	Approved    bool   `json:"approved,omitempty"`
	ApprovedBy  string `json:"approved_by,omitempty"`
	Comment     string `json:"comment,omitempty"`
}
