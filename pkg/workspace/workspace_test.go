package workspace

import (
	"context"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
)

func TestNew(t *testing.T) {
	ws := New(Config{
		BaseDir: "/tmp/test-workspace",
		Pattern: "{platform}/{chatID}",
	})

	if ws == nil {
		t.Fatal("New() returned nil")
	}
}

func TestNewWithDefaults(t *testing.T) {
	ws := New(Config{})

	fsws, ok := ws.(*FSWorkspace)
	if !ok {
		t.Fatal("unexpected type")
	}

	if fsws.baseDir != DefaultConfig().BaseDir {
		t.Errorf("baseDir = %q, want %q", fsws.baseDir, DefaultConfig().BaseDir)
	}
	if fsws.pattern != DefaultConfig().Pattern {
		t.Errorf("pattern = %q, want %q", fsws.pattern, DefaultConfig().Pattern)
	}
}

func TestGetPath(t *testing.T) {
	tmpDir := t.TempDir()
	ws := New(Config{
		BaseDir: tmpDir,
		Pattern: "{platform}/{chatID}",
	})

	update := botcore.RequestSnapshot{
		ChatID:   "chat-123",
		SenderID: "user-456",
		Metadata: map[string]string{"platform": "wecom"},
	}

	tests := []struct {
		name     string
		pathType PathType
		want     string
	}{
		{"root", PathTypeRoot, filepath.Join(tmpDir, "wecom", "chat-123")},
		{"memory", PathTypeMemory, filepath.Join(tmpDir, "wecom", "chat-123")},
		{"files", PathTypeFiles, filepath.Join(tmpDir, "wecom", "chat-123", "files")},
		{"attachments", PathTypeAttachments, filepath.Join(tmpDir, "wecom", "chat-123", "attachments")},
		{"logs", PathTypeLogs, filepath.Join(tmpDir, "wecom", "chat-123", "logs")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ws.GetPath(update, tt.pathType)
			if got != tt.want {
				t.Errorf("GetPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestGetPathWithDefaultPlatform(t *testing.T) {
	tmpDir := t.TempDir()
	ws := New(Config{
		BaseDir: tmpDir,
		Pattern: "{platform}/{chatID}",
	})

	update := botcore.RequestSnapshot{
		ChatID:   "chat-123",
		SenderID: "user-456",
		// 无 platform metadata
	}

	got := ws.GetPath(update, PathTypeRoot)
	want := filepath.Join(tmpDir, "default", "chat-123")

	if got != want {
		t.Errorf("GetPath() = %q, want %q", got, want)
	}
}

func TestEnsurePath(t *testing.T) {
	tmpDir := t.TempDir()
	ws := New(Config{
		BaseDir: tmpDir,
		Pattern: "{platform}/{chatID}",
	})

	update := botcore.RequestSnapshot{
		ChatID:   "chat-123",
		Metadata: map[string]string{"platform": "wecom"},
	}

	path, err := ws.EnsurePath(update, PathTypeFiles)
	if err != nil {
		t.Fatalf("EnsurePath() error = %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("directory was not created: %s", path)
	}
}

func TestGetMemoryFile(t *testing.T) {
	tmpDir := t.TempDir()
	ws := New(Config{
		BaseDir: tmpDir,
		Pattern: "{platform}/{chatID}",
	})

	update := botcore.RequestSnapshot{
		ChatID:   "chat-123",
		Metadata: map[string]string{"platform": "wecom"},
	}

	got := ws.GetMemoryFile(update)
	want := filepath.Join(tmpDir, "wecom", "chat-123", "CLAUDE.md")

	if got != want {
		t.Errorf("GetMemoryFile() = %q, want %q", got, want)
	}
}

func TestGetGlobalPath(t *testing.T) {
	tmpDir := t.TempDir()
	ws := New(Config{
		BaseDir: tmpDir,
		Pattern: "{platform}/{chatID}",
	})

	tests := []struct {
		name     string
		pathType PathType
		want     string
	}{
		{"root", PathTypeRoot, filepath.Join(tmpDir, "global")},
		{"files", PathTypeFiles, filepath.Join(tmpDir, "global", "files")},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := ws.GetGlobalPath(tt.pathType)
			if got != tt.want {
				t.Errorf("GetGlobalPath() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestEnsureGlobalPath(t *testing.T) {
	tmpDir := t.TempDir()
	ws := New(Config{
		BaseDir: tmpDir,
	})

	path, err := ws.EnsureGlobalPath(PathTypeFiles)
	if err != nil {
		t.Fatalf("EnsureGlobalPath() error = %v", err)
	}

	if _, err := os.Stat(path); os.IsNotExist(err) {
		t.Errorf("global directory was not created: %s", path)
	}
}

func TestListWorkspaces(t *testing.T) {
	tmpDir := t.TempDir()
	ws := New(Config{
		BaseDir: tmpDir,
		Pattern: "{platform}/{chatID}",
	})

	// 创建测试工作空间
	updates := []botcore.RequestSnapshot{
		{ChatID: "chat-1", Metadata: map[string]string{"platform": "wecom"}},
		{ChatID: "chat-2", Metadata: map[string]string{"platform": "wecom"}},
		{ChatID: "chat-3", Metadata: map[string]string{"platform": "telegram"}},
	}

	for _, update := range updates {
		path, _ := ws.EnsurePath(update, PathTypeRoot)
		// 创建 CLAUDE.md 作为工作空间标识
		os.WriteFile(filepath.Join(path, "CLAUDE.md"), []byte("# Memory"), 0644)
	}

	// 创建全局目录（应被忽略）
	globalPath, _ := ws.EnsureGlobalPath(PathTypeRoot)
	os.WriteFile(filepath.Join(globalPath, "CLAUDE.md"), []byte("# Global"), 0644)

	workspaces, err := ws.ListWorkspaces(context.Background())
	if err != nil {
		t.Fatalf("ListWorkspaces() error = %v", err)
	}

	if len(workspaces) != 3 {
		t.Errorf("ListWorkspaces() returned %d workspaces, want 3", len(workspaces))
	}
}

func TestCleanupOld(t *testing.T) {
	tmpDir := t.TempDir()
	ws := New(Config{
		BaseDir: tmpDir,
		Pattern: "{platform}/{chatID}",
	})

	// 创建测试工作空间
	update := botcore.RequestSnapshot{
		ChatID:   "old-chat",
		Metadata: map[string]string{"platform": "wecom"},
	}

	path, _ := ws.EnsurePath(update, PathTypeRoot)
	memoryFile := filepath.Join(path, "CLAUDE.md")
	os.WriteFile(memoryFile, []byte("# Old Memory"), 0644)

	// 修改文件时间为过去
	oldTime := time.Now().Add(-48 * time.Hour)
	os.Chtimes(memoryFile, oldTime, oldTime)

	// 清理 24 小时前的数据
	err := ws.CleanupOld(context.Background(), 24*time.Hour)
	if err != nil {
		t.Fatalf("CleanupOld() error = %v", err)
	}

	// 验证目录被删除
	if _, err := os.Stat(path); !os.IsNotExist(err) {
		t.Errorf("old workspace was not cleaned up: %s", path)
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"normal", "normal"},
		{"with/slash", "with_slash"},
		{"with\\backslash", "with_backslash"},
		{"with..dots", "with_dots"},
		{"with:colon", "with_colon"},
		{"with*star", "with_star"},
		{"", "_empty_"},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := sanitizePath(tt.input)
			if got != tt.want {
				t.Errorf("sanitizePath(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}
