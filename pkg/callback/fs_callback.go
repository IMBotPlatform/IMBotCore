package callback

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/IMBotPlatform/IMBotCore/pkg/scheduler"
	"github.com/google/uuid"
)

// FSCallback 基于文件系统 IPC 的回调实现
// 兼容 NanoClaw 的 IPC 模式，通过 JSON 文件进行进程间通信。
type FSCallback struct {
	ipcDir    string
	scheduler scheduler.Scheduler
}

// NewFSCallback 创建文件系统回调实例
// 参数：
//   - ipcDir: IPC 目录路径
//   - sched: 调度器实例（可选，用于任务管理）
//
// 返回：Callback 实现
func NewFSCallback(ipcDir string, sched scheduler.Scheduler) *FSCallback {
	// 确保目录存在
	os.MkdirAll(filepath.Join(ipcDir, "messages"), 0755)
	os.MkdirAll(filepath.Join(ipcDir, "tasks"), 0755)
	os.MkdirAll(filepath.Join(ipcDir, "approvals"), 0755)

	return &FSCallback{
		ipcDir:    ipcDir,
		scheduler: sched,
	}
}

// SendMessage 发送消息到指定会话
func (c *FSCallback) SendMessage(ctx context.Context, req SendMessageRequest) error {
	msg := IPCMessage{
		Type:     MessageTypeSend,
		ID:       uuid.New().String(),
		ChatID:   req.ChatID,
		Platform: req.Platform,
		Text:     req.Text,
		ReplyTo:  req.ReplyTo,
	}

	data, err := json.MarshalIndent(msg, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal message: %w", err)
	}

	filename := fmt.Sprintf("%d-%s.json", time.Now().UnixNano(), msg.ID[:8])
	filePath := filepath.Join(c.ipcDir, "messages", filename)

	if err := os.WriteFile(filePath, data, 0644); err != nil {
		return fmt.Errorf("write message file: %w", err)
	}

	return nil
}

// ScheduleTask 创建定时任务
func (c *FSCallback) ScheduleTask(ctx context.Context, req scheduler.CreateTaskRequest) (*scheduler.Task, error) {
	if c.scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured")
	}
	return c.scheduler.Create(ctx, req)
}

// PauseTask 暂停任务
func (c *FSCallback) PauseTask(ctx context.Context, taskID string) error {
	if c.scheduler == nil {
		return fmt.Errorf("scheduler not configured")
	}
	return c.scheduler.Pause(ctx, taskID)
}

// ResumeTask 恢复任务
func (c *FSCallback) ResumeTask(ctx context.Context, taskID string) error {
	if c.scheduler == nil {
		return fmt.Errorf("scheduler not configured")
	}
	return c.scheduler.Resume(ctx, taskID)
}

// CancelTask 取消任务
func (c *FSCallback) CancelTask(ctx context.Context, taskID string) error {
	if c.scheduler == nil {
		return fmt.Errorf("scheduler not configured")
	}
	return c.scheduler.Delete(ctx, taskID)
}

// ListTasks 列出群组的任务
func (c *FSCallback) ListTasks(ctx context.Context, groupID string) ([]scheduler.Task, error) {
	if c.scheduler == nil {
		return nil, fmt.Errorf("scheduler not configured")
	}
	return c.scheduler.ListByGroup(ctx, groupID)
}

// RequestApproval 请求人工审批
// 写入审批请求文件，轮询等待审批响应。
func (c *FSCallback) RequestApproval(ctx context.Context, req ApprovalRequest) (*ApprovalResponse, error) {
	approvalID := uuid.New().String()

	// 写入审批请求
	approvalReq := IPCMessage{
		Type:        MessageTypeApprovalRequest,
		ID:          approvalID,
		ChatID:      req.ChatID,
		Title:       req.Title,
		Description: req.Description,
	}

	data, err := json.MarshalIndent(approvalReq, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal approval request: %w", err)
	}

	reqFile := filepath.Join(c.ipcDir, "approvals", approvalID+".request.json")
	if err := os.WriteFile(reqFile, data, 0644); err != nil {
		return nil, fmt.Errorf("write approval request: %w", err)
	}

	// 轮询等待响应
	respFile := filepath.Join(c.ipcDir, "approvals", approvalID+".response.json")
	timeout := req.Timeout
	if timeout == 0 {
		timeout = 5 * time.Minute // 默认 5 分钟超时
	}

	deadline := time.Now().Add(timeout)
	pollInterval := time.Second

	for time.Now().Before(deadline) {
		select {
		case <-ctx.Done():
			// 清理请求文件
			os.Remove(reqFile)
			return nil, ctx.Err()
		default:
		}

		// 检查响应文件
		respData, err := os.ReadFile(respFile)
		if err == nil {
			var resp IPCMessage
			if err := json.Unmarshal(respData, &resp); err == nil {
				// 清理文件
				os.Remove(reqFile)
				os.Remove(respFile)

				return &ApprovalResponse{
					Approved:   resp.Approved,
					ApprovedBy: resp.ApprovedBy,
					Comment:    resp.Comment,
				}, nil
			}
		}

		time.Sleep(pollInterval)
	}

	// 超时，清理请求文件
	os.Remove(reqFile)
	return nil, fmt.Errorf("approval timeout after %v", timeout)
}

// ReadPendingMessages 读取并删除待处理的消息
// 用于主程序轮询 IPC 目录。
// 参数：ctx - 上下文
// 返回：消息列表和可能的错误
func (c *FSCallback) ReadPendingMessages(ctx context.Context) ([]IPCMessage, error) {
	msgDir := filepath.Join(c.ipcDir, "messages")

	entries, err := os.ReadDir(msgDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var messages []IPCMessage
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".json" {
			continue
		}

		filePath := filepath.Join(msgDir, entry.Name())
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var msg IPCMessage
		if err := json.Unmarshal(data, &msg); err == nil {
			messages = append(messages, msg)
			// 读取后删除
			os.Remove(filePath)
		}
	}

	return messages, nil
}

// ReadPendingApprovalRequests 读取待处理的审批请求
// 用于主程序轮询审批请求。
// 参数：ctx - 上下文
// 返回：审批请求列表和可能的错误
func (c *FSCallback) ReadPendingApprovalRequests(ctx context.Context) ([]IPCMessage, error) {
	approvalDir := filepath.Join(c.ipcDir, "approvals")

	entries, err := os.ReadDir(approvalDir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, err
	}

	var requests []IPCMessage
	for _, entry := range entries {
		if entry.IsDir() {
			continue
		}

		// 只读取 .request.json 文件
		name := entry.Name()
		if len(name) < 13 || name[len(name)-13:] != ".request.json" {
			continue
		}

		filePath := filepath.Join(approvalDir, name)
		data, err := os.ReadFile(filePath)
		if err != nil {
			continue
		}

		var msg IPCMessage
		if err := json.Unmarshal(data, &msg); err == nil {
			requests = append(requests, msg)
		}
	}

	return requests, nil
}

// RespondToApproval 响应审批请求
// 参数：
//   - ctx: 上下文
//   - approvalID: 审批请求 ID
//   - approved: 是否批准
//   - approvedBy: 批准人
//   - comment: 批注
//
// 返回：可能的错误
func (c *FSCallback) RespondToApproval(ctx context.Context, approvalID string, approved bool, approvedBy, comment string) error {
	resp := IPCMessage{
		Type:       MessageTypeApprovalResponse,
		ID:         approvalID,
		Approved:   approved,
		ApprovedBy: approvedBy,
		Comment:    comment,
	}

	data, err := json.MarshalIndent(resp, "", "  ")
	if err != nil {
		return fmt.Errorf("marshal approval response: %w", err)
	}

	respFile := filepath.Join(c.ipcDir, "approvals", approvalID+".response.json")
	return os.WriteFile(respFile, data, 0644)
}
