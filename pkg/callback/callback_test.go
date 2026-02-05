package callback

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IMBotPlatform/IMBotCore/pkg/scheduler"
)

func TestNewFSCallback(t *testing.T) {
	tmpDir := t.TempDir()
	ipcDir := filepath.Join(tmpDir, "ipc")

	cb := NewFSCallback(ipcDir, nil)
	if cb == nil {
		t.Fatal("NewFSCallback() returned nil")
	}

	// 验证目录创建
	dirs := []string{"messages", "tasks", "approvals"}
	for _, dir := range dirs {
		path := filepath.Join(ipcDir, dir)
		if _, err := os.Stat(path); os.IsNotExist(err) {
			t.Errorf("directory %s was not created", dir)
		}
	}
}

func TestSendMessage(t *testing.T) {
	tmpDir := t.TempDir()
	ipcDir := filepath.Join(tmpDir, "ipc")
	cb := NewFSCallback(ipcDir, nil)

	ctx := context.Background()

	err := cb.SendMessage(ctx, SendMessageRequest{
		ChatID:   "chat-123",
		Platform: "wecom",
		Text:     "Hello, World!",
	})

	if err != nil {
		t.Fatalf("SendMessage() error = %v", err)
	}

	// 验证消息文件创建
	entries, _ := os.ReadDir(filepath.Join(ipcDir, "messages"))
	if len(entries) != 1 {
		t.Errorf("expected 1 message file, got %d", len(entries))
	}
}

func TestReadPendingMessages(t *testing.T) {
	tmpDir := t.TempDir()
	ipcDir := filepath.Join(tmpDir, "ipc")
	cb := NewFSCallback(ipcDir, nil)

	ctx := context.Background()

	// 发送多条消息
	for i := 0; i < 3; i++ {
		cb.SendMessage(ctx, SendMessageRequest{
			ChatID: "chat-123",
			Text:   "Message " + string(rune('A'+i)),
		})
	}

	// 读取消息
	messages, err := cb.ReadPendingMessages(ctx)
	if err != nil {
		t.Fatalf("ReadPendingMessages() error = %v", err)
	}

	if len(messages) != 3 {
		t.Errorf("ReadPendingMessages() returned %d messages, want 3", len(messages))
	}

	// 验证消息被删除
	entries, _ := os.ReadDir(filepath.Join(ipcDir, "messages"))
	if len(entries) != 0 {
		t.Errorf("expected 0 message files after read, got %d", len(entries))
	}
}

func TestScheduleTaskWithScheduler(t *testing.T) {
	tmpDir := t.TempDir()
	ipcDir := filepath.Join(tmpDir, "ipc")
	dbPath := filepath.Join(tmpDir, "scheduler.db")

	sched, err := scheduler.New(scheduler.Config{DBPath: dbPath})
	if err != nil {
		t.Fatalf("create scheduler error = %v", err)
	}
	defer sched.Stop()

	cb := NewFSCallback(ipcDir, sched)
	ctx := context.Background()

	task, err := cb.ScheduleTask(ctx, scheduler.CreateTaskRequest{
		GroupID:       "group-1",
		ChatID:        "chat-1",
		Prompt:        "测试任务",
		ScheduleType:  scheduler.ScheduleTypeInterval,
		ScheduleValue: "60000",
	})

	if err != nil {
		t.Fatalf("ScheduleTask() error = %v", err)
	}

	if task.ID == "" {
		t.Error("task.ID is empty")
	}
}

func TestScheduleTaskWithoutScheduler(t *testing.T) {
	tmpDir := t.TempDir()
	ipcDir := filepath.Join(tmpDir, "ipc")
	cb := NewFSCallback(ipcDir, nil) // 无调度器

	ctx := context.Background()

	_, err := cb.ScheduleTask(ctx, scheduler.CreateTaskRequest{
		GroupID:       "group-1",
		ChatID:        "chat-1",
		Prompt:        "测试任务",
		ScheduleType:  scheduler.ScheduleTypeInterval,
		ScheduleValue: "60000",
	})

	if err == nil {
		t.Error("expected error without scheduler, got nil")
	}
}

func TestPauseResumeTask(t *testing.T) {
	tmpDir := t.TempDir()
	ipcDir := filepath.Join(tmpDir, "ipc")
	dbPath := filepath.Join(tmpDir, "scheduler.db")

	sched, _ := scheduler.New(scheduler.Config{DBPath: dbPath})
	defer sched.Stop()

	cb := NewFSCallback(ipcDir, sched)
	ctx := context.Background()

	task, _ := cb.ScheduleTask(ctx, scheduler.CreateTaskRequest{
		GroupID:       "group-1",
		ChatID:        "chat-1",
		Prompt:        "可暂停任务",
		ScheduleType:  scheduler.ScheduleTypeInterval,
		ScheduleValue: "60000",
	})

	// Pause
	if err := cb.PauseTask(ctx, task.ID); err != nil {
		t.Fatalf("PauseTask() error = %v", err)
	}

	// Resume
	if err := cb.ResumeTask(ctx, task.ID); err != nil {
		t.Fatalf("ResumeTask() error = %v", err)
	}
}

func TestCancelTask(t *testing.T) {
	tmpDir := t.TempDir()
	ipcDir := filepath.Join(tmpDir, "ipc")
	dbPath := filepath.Join(tmpDir, "scheduler.db")

	sched, _ := scheduler.New(scheduler.Config{DBPath: dbPath})
	defer sched.Stop()

	cb := NewFSCallback(ipcDir, sched)
	ctx := context.Background()

	task, _ := cb.ScheduleTask(ctx, scheduler.CreateTaskRequest{
		GroupID:       "group-1",
		ChatID:        "chat-1",
		Prompt:        "待取消任务",
		ScheduleType:  scheduler.ScheduleTypeInterval,
		ScheduleValue: "60000",
	})

	if err := cb.CancelTask(ctx, task.ID); err != nil {
		t.Fatalf("CancelTask() error = %v", err)
	}

	// 验证任务被删除
	tasks, _ := cb.ListTasks(ctx, "group-1")
	if len(tasks) != 0 {
		t.Errorf("expected 0 tasks after cancel, got %d", len(tasks))
	}
}

func TestListTasks(t *testing.T) {
	tmpDir := t.TempDir()
	ipcDir := filepath.Join(tmpDir, "ipc")
	dbPath := filepath.Join(tmpDir, "scheduler.db")

	sched, _ := scheduler.New(scheduler.Config{DBPath: dbPath})
	defer sched.Stop()

	cb := NewFSCallback(ipcDir, sched)
	ctx := context.Background()

	// 创建多个任务
	cb.ScheduleTask(ctx, scheduler.CreateTaskRequest{GroupID: "group-1", ChatID: "chat-1", Prompt: "任务1", ScheduleType: scheduler.ScheduleTypeInterval, ScheduleValue: "60000"})
	cb.ScheduleTask(ctx, scheduler.CreateTaskRequest{GroupID: "group-1", ChatID: "chat-1", Prompt: "任务2", ScheduleType: scheduler.ScheduleTypeInterval, ScheduleValue: "60000"})
	cb.ScheduleTask(ctx, scheduler.CreateTaskRequest{GroupID: "group-2", ChatID: "chat-2", Prompt: "任务3", ScheduleType: scheduler.ScheduleTypeInterval, ScheduleValue: "60000"})

	tasks, err := cb.ListTasks(ctx, "group-1")
	if err != nil {
		t.Fatalf("ListTasks() error = %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("ListTasks() returned %d tasks, want 2", len(tasks))
	}
}

func TestApprovalFlow(t *testing.T) {
	tmpDir := t.TempDir()
	ipcDir := filepath.Join(tmpDir, "ipc")
	cb := NewFSCallback(ipcDir, nil)

	ctx := context.Background()

	// 模拟审批流程
	approvalDone := make(chan struct{})
	var approvalResult *ApprovalResponse
	var approvalErr error

	go func() {
		approvalResult, approvalErr = cb.RequestApproval(ctx, ApprovalRequest{
			ChatID:      "chat-123",
			Title:       "测试审批",
			Description: "这是一个测试审批请求",
			Timeout:     2 * time.Second,
		})
		close(approvalDone)
	}()

	// 等待请求文件出现
	time.Sleep(100 * time.Millisecond)

	// 读取审批请求
	requests, err := cb.ReadPendingApprovalRequests(ctx)
	if err != nil {
		t.Fatalf("ReadPendingApprovalRequests() error = %v", err)
	}

	if len(requests) != 1 {
		t.Fatalf("expected 1 approval request, got %d", len(requests))
	}

	// 响应审批
	if err := cb.RespondToApproval(ctx, requests[0].ID, true, "admin", "同意"); err != nil {
		t.Fatalf("RespondToApproval() error = %v", err)
	}

	// 等待审批完成
	<-approvalDone

	if approvalErr != nil {
		t.Fatalf("RequestApproval() error = %v", approvalErr)
	}

	if !approvalResult.Approved {
		t.Error("expected approval to be approved")
	}

	if approvalResult.ApprovedBy != "admin" {
		t.Errorf("ApprovedBy = %q, want %q", approvalResult.ApprovedBy, "admin")
	}
}

func TestApprovalTimeout(t *testing.T) {
	tmpDir := t.TempDir()
	ipcDir := filepath.Join(tmpDir, "ipc")
	cb := NewFSCallback(ipcDir, nil)

	ctx := context.Background()

	_, err := cb.RequestApproval(ctx, ApprovalRequest{
		ChatID:  "chat-123",
		Title:   "超时审批",
		Timeout: 100 * time.Millisecond,
	})

	if err == nil {
		t.Error("expected timeout error, got nil")
	}
}

func TestApprovalCancel(t *testing.T) {
	tmpDir := t.TempDir()
	ipcDir := filepath.Join(tmpDir, "ipc")
	cb := NewFSCallback(ipcDir, nil)

	ctx, cancel := context.WithCancel(context.Background())

	go func() {
		time.Sleep(100 * time.Millisecond)
		cancel()
	}()

	_, err := cb.RequestApproval(ctx, ApprovalRequest{
		ChatID:  "chat-123",
		Title:   "取消审批",
		Timeout: 10 * time.Second,
	})

	if err != context.Canceled {
		t.Errorf("expected context.Canceled, got %v", err)
	}
}
