// Package container 提供容器隔离执行能力。
// 允许 AI Agent 在独立容器中执行，通过文件系统 IPC 与主进程通信。
package container

import (
	"context"
	"time"
)

// RunRequest 容器执行请求。
type RunRequest struct {
	ChatID       string            // 会话 ID
	SessionID    string            // Claude CLI session ID
	IsNewSession bool              // 是否新会话
	Prompt       string            // 执行提示词
	WorkspaceDir string            // 工作空间目录
	SessionsDir  string            // Claude sessions 目录
	IPCDir       string            // IPC 通信目录
	GlobalDir    string            // 全局记忆目录（只读）
	EnvVars      map[string]string // 环境变量（已过滤）
	Timeout      time.Duration     // 执行超时
	IsMain       bool              // Main 组有更多权限
	Metadata     map[string]string // 扩展元数据
}

// RunResult 容器执行结果。
type RunResult struct {
	Status       string        // success / error
	Output       string        // 输出内容
	NewSessionID string        // 新会话 ID（如有变化）
	Duration     time.Duration // 执行耗时
	ExitCode     int           // 容器退出码
	Error        string        // 错误信息（如有）
}

// Runner 容器执行器接口。
type Runner interface {
	// Run 在容器中执行 prompt。
	// 返回执行结果和错误。
	Run(ctx context.Context, req RunRequest) (*RunResult, error)

	// Stop 停止指定容器。
	Stop(containerID string) error

	// Cleanup 清理过期容器资源。
	Cleanup(ctx context.Context) error

	// Close 关闭执行器，释放资源。
	Close() error
}

// Config 容器执行器配置。
type Config struct {
	Image          string        // 容器镜像名
	MemoryLimit    int64         // 内存限制 (bytes)
	CPUQuota       int64         // CPU 配额
	NetworkMode    string        // 网络模式 (bridge/host/none)
	DefaultTimeout time.Duration // 默认超时时间
	MaxOutputSize  int           // 最大输出大小 (bytes)
	AllowedEnvVars []string      // 允许的环境变量列表
	DockerHost     string        // Docker API 地址（可选）
}

// DefaultConfig 返回默认配置。
func DefaultConfig() Config {
	return Config{
		Image:          "claude-code-agent:latest",
		MemoryLimit:    512 * 1024 * 1024, // 512MB
		CPUQuota:       100000,            // 1 CPU
		NetworkMode:    "bridge",
		DefaultTimeout: 5 * time.Minute,
		MaxOutputSize:  1024 * 1024, // 1MB
		AllowedEnvVars: []string{
			"ANTHROPIC_API_KEY",
			"CLAUDE_CODE_OAUTH_TOKEN",
		},
	}
}

// VolumeMount 卷挂载配置。
type VolumeMount struct {
	HostPath      string // 宿主机路径
	ContainerPath string // 容器内路径
	ReadOnly      bool   // 是否只读
}
