// Package workspace 提供会话工作空间管理功能。
// 为每个会话（用户/群组组合）提供独立的文件系统工作空间。
package workspace

import (
	"context"
	"time"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
)

// PathType 路径类型枚举
type PathType string

const (
	// PathTypeRoot 工作空间根目录
	PathTypeRoot PathType = "root"
	// PathTypeMemory CLAUDE.md 所在目录
	PathTypeMemory PathType = "memory"
	// PathTypeFiles 用户文件目录
	PathTypeFiles PathType = "files"
	// PathTypeAttachments 附件目录
	PathTypeAttachments PathType = "attachments"
	// PathTypeLogs 日志目录
	PathTypeLogs PathType = "logs"
)

// Workspace 会话工作空间接口
// 提供会话级别的文件系统隔离，每个会话拥有独立的工作目录。
type Workspace interface {
	// GetPath 获取指定类型的路径（不创建目录）
	// 参数：
	//   - update: 请求快照，用于提取会话标识
	//   - pathType: 路径类型
	// 返回：路径字符串
	GetPath(update botcore.RequestSnapshot, pathType PathType) string

	// EnsurePath 确保目录存在并返回路径
	// 参数：
	//   - update: 请求快照
	//   - pathType: 路径类型
	// 返回：路径字符串和可能的错误
	EnsurePath(update botcore.RequestSnapshot, pathType PathType) (string, error)

	// GetMemoryFile 获取 CLAUDE.md 文件路径
	// 参数：update - 请求快照
	// 返回：CLAUDE.md 文件的完整路径
	GetMemoryFile(update botcore.RequestSnapshot) string

	// GetGlobalPath 获取全局共享路径
	// 参数：pathType - 路径类型
	// 返回：全局目录路径
	GetGlobalPath(pathType PathType) string

	// EnsureGlobalPath 确保全局目录存在
	// 参数：pathType - 路径类型
	// 返回：全局目录路径和可能的错误
	EnsureGlobalPath(pathType PathType) (string, error)

	// ListWorkspaces 列出所有工作空间（用于管理）
	// 参数：ctx - 上下文
	// 返回：工作空间信息列表和可能的错误
	ListWorkspaces(ctx context.Context) ([]WorkspaceInfo, error)

	// CleanupOld 清理过期工作空间
	// 参数：
	//   - ctx: 上下文
	//   - olderThan: 清理多久之前的工作空间
	// 返回：可能的错误
	CleanupOld(ctx context.Context, olderThan time.Duration) error
}

// WorkspaceInfo 工作空间元信息
type WorkspaceInfo struct {
	Platform   string    // 平台标识
	ChatID     string    // 会话 ID
	Path       string    // 工作空间路径
	CreatedAt  time.Time // 创建时间
	LastAccess time.Time // 最后访问时间
	SizeBytes  int64     // 占用空间（字节）
}

// Config 工作空间配置
type Config struct {
	// BaseDir 基础目录，默认 /data/workspaces
	BaseDir string
	// Pattern 路径模式，支持占位符：{platform}, {chatID}, {senderID}
	// 默认 "{platform}/{chatID}"
	Pattern string
}

// DefaultConfig 返回默认配置
func DefaultConfig() Config {
	return Config{
		BaseDir: "/data/workspaces",
		Pattern: "{platform}/{chatID}",
	}
}
