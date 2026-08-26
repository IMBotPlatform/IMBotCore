package container

import (
	"os"
	"path/filepath"
	"testing"
)

func TestMountValidatorBlockedPatterns(t *testing.T) {
	validator := NewMountValidator(nil)

	// 测试被禁止的路径
	blockedPaths := []string{
		filepath.Join(os.Getenv("HOME"), ".ssh"),
		filepath.Join(os.Getenv("HOME"), ".aws"),
		filepath.Join(os.Getenv("HOME"), ".gnupg"),
		"/path/to/.secret/file",
		"/path/to/credentials",
	}

	for _, p := range blockedPaths {
		// 跳过不存在的路径
		if _, err := os.Stat(p); os.IsNotExist(err) {
			continue
		}

		result := validator.Validate(p)
		if result.Allowed {
			t.Errorf("expected %s to be blocked", p)
		}
	}
}

func TestMountValidatorAllowedPaths(t *testing.T) {
	validator := NewMountValidator(nil)

	// 创建临时目录测试
	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "allowed_dir")
	os.MkdirAll(testPath, 0755)

	result := validator.Validate(testPath)
	if !result.Allowed {
		t.Errorf("expected %s to be allowed, got: %s", testPath, result.Reason)
	}
	// macOS: /var -> /private/var，所以需要比较真实路径
	expectedPath, _ := filepath.EvalSymlinks(testPath)
	if result.RealPath != expectedPath {
		t.Errorf("expected realPath=%s, got=%s", expectedPath, result.RealPath)
	}
}

func TestMountValidatorNonExistentPath(t *testing.T) {
	validator := NewMountValidator(nil)

	result := validator.Validate("/nonexistent/path/12345")
	if result.Allowed {
		t.Error("expected nonexistent path to be blocked")
	}
	if result.Reason == "" {
		t.Error("expected reason to be set")
	}
}

func TestMountValidatorCustomBlockedPatterns(t *testing.T) {
	validator := NewMountValidator([]string{"custom_blocked"})

	tmpDir := t.TempDir()
	testPath := filepath.Join(tmpDir, "custom_blocked_dir")
	os.MkdirAll(testPath, 0755)

	result := validator.Validate(testPath)
	if result.Allowed {
		t.Error("expected custom blocked pattern to work")
	}
}

func TestValidateMounts(t *testing.T) {
	validator := NewMountValidator(nil)

	tmpDir := t.TempDir()
	allowedPath := filepath.Join(tmpDir, "allowed")
	os.MkdirAll(allowedPath, 0755)

	mounts := []VolumeMount{
		{HostPath: allowedPath, ContainerPath: "/workspace/test"},
		{HostPath: "/nonexistent/path", ContainerPath: "/workspace/bad"},
	}

	allowed, rejected := validator.ValidateMounts(mounts)

	if len(allowed) != 1 {
		t.Errorf("expected 1 allowed mount, got %d", len(allowed))
	}
	if len(rejected) != 1 {
		t.Errorf("expected 1 rejected mount, got %d", len(rejected))
	}
}

func TestSanitizePath(t *testing.T) {
	tests := []struct {
		input    string
		expected string
	}{
		{"normal", "normal"},
		{"path/with/slashes", "path_with_slashes"},
		{"path..escape", "path_escape"},
		{"windows:drive", "windows_drive"},
	}

	for _, tt := range tests {
		result := SanitizePath(tt.input)
		if result != tt.expected {
			t.Errorf("SanitizePath(%q) = %q, want %q", tt.input, result, tt.expected)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.Image != "" || cfg.WorkingDir != "" || cfg.StateMountPath != "" {
		t.Error("platform-neutral defaults must not select an image, working directory, or runtime state path")
	}
	if cfg.MemoryLimit == 0 {
		t.Error("expected memory limit to be set")
	}
	if len(cfg.AllowedEnvVars) != 0 {
		t.Error("platform-neutral defaults must not allow provider credentials")
	}
}

func TestNewDockerRunnerRequiresExplicitRuntimeConfiguration(t *testing.T) {
	if _, err := NewDockerRunner(DefaultConfig()); err == nil {
		t.Fatal("expected empty platform-neutral config to be rejected")
	}
}

func TestBuildMountsUsesExplicitRuntimePaths(t *testing.T) {
	runner := &DockerRunner{config: Config{
		WorkspaceMountPath: "/runtime/workspace",
		StateMountPath:     "/runtime/state",
		IPCMountPath:       "/runtime/ipc",
		GlobalMountPath:    "/runtime/global",
	}}
	mounts, err := runner.buildMounts(RunRequest{
		WorkspaceDir: "/host/workspace",
		SessionsDir:  "/host/state",
		IPCDir:       "/host/ipc",
		GlobalDir:    "/host/global",
	})
	if err != nil {
		t.Fatalf("build mounts: %v", err)
	}
	if len(mounts) != 4 {
		t.Fatalf("expected 4 mounts, got %d", len(mounts))
	}
	if mounts[0].Target != "/runtime/workspace" || mounts[1].Target != "/runtime/state" {
		t.Fatalf("unexpected runtime mount targets: %#v", mounts)
	}
	if !mounts[3].ReadOnly {
		t.Fatal("global mount must stay read-only")
	}

	runner.config.StateMountPath = ""
	if _, err := runner.buildMounts(RunRequest{SessionsDir: "/host/state"}); err == nil {
		t.Fatal("expected missing runtime state target to be rejected")
	}
}
