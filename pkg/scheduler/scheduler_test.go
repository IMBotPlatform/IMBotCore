package scheduler

import (
	"context"
	"os"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"

	sched, err := New(Config{
		DBPath:       tmpFile,
		PollInterval: time.Second,
	})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	defer sched.Stop()

	if sched == nil {
		t.Fatal("New() returned nil")
	}
}

func TestCreateTask(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	sched, _ := New(Config{DBPath: tmpFile})
	defer sched.Stop()

	ctx := context.Background()

	task, err := sched.Create(ctx, CreateTaskRequest{
		GroupID:       "group-1",
		ChatID:        "chat-1",
		Platform:      "wecom",
		Prompt:        "每日任务",
		ScheduleType:  ScheduleTypeInterval,
		ScheduleValue: "3600000", // 1 小时
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if task.ID == "" {
		t.Error("task.ID is empty")
	}
	if task.Status != TaskStatusActive {
		t.Errorf("task.Status = %v, want %v", task.Status, TaskStatusActive)
	}
	if task.NextRun == nil {
		t.Error("task.NextRun is nil")
	}
}

func TestCreateCronTask(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	sched, _ := New(Config{DBPath: tmpFile})
	defer sched.Stop()

	ctx := context.Background()

	task, err := sched.Create(ctx, CreateTaskRequest{
		GroupID:       "group-1",
		ChatID:        "chat-1",
		Platform:      "wecom",
		Prompt:        "每天 9 点执行",
		ScheduleType:  ScheduleTypeCron,
		ScheduleValue: "0 9 * * *",
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if task.NextRun == nil {
		t.Error("task.NextRun is nil")
	}
}

func TestCreateOnceTask(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	sched, _ := New(Config{DBPath: tmpFile})
	defer sched.Stop()

	ctx := context.Background()

	futureTime := time.Now().Add(time.Hour).Format(time.RFC3339)

	task, err := sched.Create(ctx, CreateTaskRequest{
		GroupID:       "group-1",
		ChatID:        "chat-1",
		Platform:      "wecom",
		Prompt:        "一次性任务",
		ScheduleType:  ScheduleTypeOnce,
		ScheduleValue: futureTime,
	})

	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	if task.NextRun == nil {
		t.Error("task.NextRun is nil")
	}
}

func TestGetTask(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	sched, _ := New(Config{DBPath: tmpFile})
	defer sched.Stop()

	ctx := context.Background()

	created, _ := sched.Create(ctx, CreateTaskRequest{
		GroupID:       "group-1",
		ChatID:        "chat-1",
		Prompt:        "测试任务",
		ScheduleType:  ScheduleTypeInterval,
		ScheduleValue: "60000",
	})

	got, err := sched.Get(ctx, created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}

	if got.ID != created.ID {
		t.Errorf("Get().ID = %v, want %v", got.ID, created.ID)
	}
	if got.Prompt != created.Prompt {
		t.Errorf("Get().Prompt = %v, want %v", got.Prompt, created.Prompt)
	}
}

func TestDeleteTask(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	sched, _ := New(Config{DBPath: tmpFile})
	defer sched.Stop()

	ctx := context.Background()

	task, _ := sched.Create(ctx, CreateTaskRequest{
		GroupID:       "group-1",
		ChatID:        "chat-1",
		Prompt:        "待删除",
		ScheduleType:  ScheduleTypeInterval,
		ScheduleValue: "60000",
	})

	if err := sched.Delete(ctx, task.ID); err != nil {
		t.Fatalf("Delete() error = %v", err)
	}

	_, err := sched.Get(ctx, task.ID)
	if err == nil {
		t.Error("expected error after delete, got nil")
	}
}

func TestPauseResumeTask(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	sched, _ := New(Config{DBPath: tmpFile})
	defer sched.Stop()

	ctx := context.Background()

	task, _ := sched.Create(ctx, CreateTaskRequest{
		GroupID:       "group-1",
		ChatID:        "chat-1",
		Prompt:        "可暂停任务",
		ScheduleType:  ScheduleTypeInterval,
		ScheduleValue: "60000",
	})

	// Pause
	if err := sched.Pause(ctx, task.ID); err != nil {
		t.Fatalf("Pause() error = %v", err)
	}

	paused, _ := sched.Get(ctx, task.ID)
	if paused.Status != TaskStatusPaused {
		t.Errorf("after Pause, Status = %v, want %v", paused.Status, TaskStatusPaused)
	}

	// Resume
	if err := sched.Resume(ctx, task.ID); err != nil {
		t.Fatalf("Resume() error = %v", err)
	}

	resumed, _ := sched.Get(ctx, task.ID)
	if resumed.Status != TaskStatusActive {
		t.Errorf("after Resume, Status = %v, want %v", resumed.Status, TaskStatusActive)
	}
}

func TestListByGroup(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	sched, _ := New(Config{DBPath: tmpFile})
	defer sched.Stop()

	ctx := context.Background()

	// 创建多个任务
	sched.Create(ctx, CreateTaskRequest{GroupID: "group-1", ChatID: "chat-1", Prompt: "任务1", ScheduleType: ScheduleTypeInterval, ScheduleValue: "60000"})
	sched.Create(ctx, CreateTaskRequest{GroupID: "group-1", ChatID: "chat-1", Prompt: "任务2", ScheduleType: ScheduleTypeInterval, ScheduleValue: "60000"})
	sched.Create(ctx, CreateTaskRequest{GroupID: "group-2", ChatID: "chat-2", Prompt: "任务3", ScheduleType: ScheduleTypeInterval, ScheduleValue: "60000"})

	tasks, err := sched.ListByGroup(ctx, "group-1")
	if err != nil {
		t.Fatalf("ListByGroup() error = %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("ListByGroup() returned %d tasks, want 2", len(tasks))
	}
}

func TestListAll(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	sched, _ := New(Config{DBPath: tmpFile})
	defer sched.Stop()

	ctx := context.Background()

	sched.Create(ctx, CreateTaskRequest{GroupID: "group-1", ChatID: "chat-1", Prompt: "任务1", ScheduleType: ScheduleTypeInterval, ScheduleValue: "60000"})
	sched.Create(ctx, CreateTaskRequest{GroupID: "group-2", ChatID: "chat-2", Prompt: "任务2", ScheduleType: ScheduleTypeInterval, ScheduleValue: "60000"})

	tasks, err := sched.ListAll(ctx)
	if err != nil {
		t.Fatalf("ListAll() error = %v", err)
	}

	if len(tasks) != 2 {
		t.Errorf("ListAll() returned %d tasks, want 2", len(tasks))
	}
}

func TestGetDueTasks(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	sched, _ := New(Config{DBPath: tmpFile})
	defer sched.Stop()

	ctx := context.Background()

	// 创建一个立即到期的任务（interval = 1ms）
	sched.Create(ctx, CreateTaskRequest{
		GroupID:       "group-1",
		ChatID:        "chat-1",
		Prompt:        "到期任务",
		ScheduleType:  ScheduleTypeInterval,
		ScheduleValue: "1", // 1ms
	})

	// 等待任务到期
	time.Sleep(10 * time.Millisecond)

	dueTasks, err := sched.GetDueTasks(ctx)
	if err != nil {
		t.Fatalf("GetDueTasks() error = %v", err)
	}

	if len(dueTasks) != 1 {
		t.Errorf("GetDueTasks() returned %d tasks, want 1", len(dueTasks))
	}
}

func TestLogRunAndGetRunLogs(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	sched, _ := New(Config{DBPath: tmpFile})
	defer sched.Stop()

	ctx := context.Background()

	task, _ := sched.Create(ctx, CreateTaskRequest{
		GroupID:       "group-1",
		ChatID:        "chat-1",
		Prompt:        "日志任务",
		ScheduleType:  ScheduleTypeInterval,
		ScheduleValue: "60000",
	})

	// 记录日志
	err := sched.LogRun(ctx, TaskRunLog{
		TaskID:     task.ID,
		RunAt:      time.Now(),
		DurationMs: 1234,
		Status:     "success",
		Result:     "执行成功",
	})
	if err != nil {
		t.Fatalf("LogRun() error = %v", err)
	}

	// 获取日志
	logs, err := sched.GetRunLogs(ctx, task.ID, 10)
	if err != nil {
		t.Fatalf("GetRunLogs() error = %v", err)
	}

	if len(logs) != 1 {
		t.Errorf("GetRunLogs() returned %d logs, want 1", len(logs))
	}

	if logs[0].DurationMs != 1234 {
		t.Errorf("log.DurationMs = %d, want 1234", logs[0].DurationMs)
	}
}

func TestUpdateAfterRun(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	sched, _ := New(Config{DBPath: tmpFile})
	defer sched.Stop()

	ctx := context.Background()

	task, _ := sched.Create(ctx, CreateTaskRequest{
		GroupID:       "group-1",
		ChatID:        "chat-1",
		Prompt:        "更新任务",
		ScheduleType:  ScheduleTypeInterval,
		ScheduleValue: "60000",
		MaxRuns:       2,
	})

	// 第一次执行
	sched.UpdateAfterRun(ctx, task.ID, "result1", nil)

	updated, _ := sched.Get(ctx, task.ID)
	if updated.RunCount != 1 {
		t.Errorf("RunCount = %d, want 1", updated.RunCount)
	}
	if updated.Status != TaskStatusActive {
		t.Errorf("Status = %v, want %v", updated.Status, TaskStatusActive)
	}

	// 第二次执行（达到最大次数）
	sched.UpdateAfterRun(ctx, task.ID, "result2", nil)

	completed, _ := sched.Get(ctx, task.ID)
	if completed.RunCount != 2 {
		t.Errorf("RunCount = %d, want 2", completed.RunCount)
	}
	if completed.Status != TaskStatusCompleted {
		t.Errorf("Status = %v, want %v", completed.Status, TaskStatusCompleted)
	}
}

func TestSchedulerStartStop(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	sched, _ := New(Config{
		DBPath:       tmpFile,
		PollInterval: 100 * time.Millisecond,
	})

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	handlerCalled := make(chan struct{}, 1)
	sched.OnDue(func(ctx context.Context, task Task) error {
		select {
		case handlerCalled <- struct{}{}:
		default:
		}
		return nil
	})

	// 创建立即到期的任务
	sched.Create(ctx, CreateTaskRequest{
		GroupID:       "group-1",
		ChatID:        "chat-1",
		Prompt:        "立即执行",
		ScheduleType:  ScheduleTypeInterval,
		ScheduleValue: "1",
	})
	time.Sleep(10 * time.Millisecond)

	if err := sched.Start(ctx); err != nil {
		t.Fatalf("Start() error = %v", err)
	}

	// 等待 handler 被调用
	select {
	case <-handlerCalled:
		// OK
	case <-time.After(time.Second):
		t.Error("handler was not called within timeout")
	}

	if err := sched.Stop(); err != nil {
		t.Fatalf("Stop() error = %v", err)
	}
}

func TestInvalidSchedule(t *testing.T) {
	tmpFile := t.TempDir() + "/test.db"
	sched, _ := New(Config{DBPath: tmpFile})
	defer sched.Stop()

	ctx := context.Background()

	// 无效的 cron 表达式
	_, err := sched.Create(ctx, CreateTaskRequest{
		GroupID:       "group-1",
		ChatID:        "chat-1",
		Prompt:        "invalid",
		ScheduleType:  ScheduleTypeCron,
		ScheduleValue: "invalid cron",
	})

	if err == nil {
		t.Error("expected error for invalid cron, got nil")
	}
}

func TestMain(m *testing.M) {
	os.Exit(m.Run())
}
