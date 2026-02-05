package workspace

import (
	"context"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/IMBotPlatform/IMBotCore/pkg/botcore"
)

// FSWorkspace 基于文件系统的工作空间实现
type FSWorkspace struct {
	baseDir string
	pattern string
}

// New 创建文件系统工作空间实例
// 参数：cfg - 工作空间配置
// 返回：Workspace 实现
func New(cfg Config) Workspace {
	baseDir := cfg.BaseDir
	if baseDir == "" {
		baseDir = DefaultConfig().BaseDir
	}
	pattern := cfg.Pattern
	if pattern == "" {
		pattern = DefaultConfig().Pattern
	}
	return &FSWorkspace{
		baseDir: baseDir,
		pattern: pattern,
	}
}

// resolvePath 根据 update 解析工作空间路径
func (w *FSWorkspace) resolvePath(update botcore.RequestSnapshot) string {
	path := w.pattern

	// 获取平台标识
	platform := "default"
	if p, ok := update.Metadata["platform"]; ok && p != "" {
		platform = sanitizePath(p)
	}

	// 替换占位符
	path = strings.ReplaceAll(path, "{platform}", platform)
	path = strings.ReplaceAll(path, "{chatID}", sanitizePath(update.ChatID))
	path = strings.ReplaceAll(path, "{senderID}", sanitizePath(update.SenderID))

	return filepath.Join(w.baseDir, path)
}

// GetPath 获取指定类型的路径（不创建目录）
func (w *FSWorkspace) GetPath(update botcore.RequestSnapshot, pathType PathType) string {
	base := w.resolvePath(update)
	return w.resolvePathType(base, pathType)
}

// resolvePathType 根据路径类型返回完整路径
func (w *FSWorkspace) resolvePathType(base string, pathType PathType) string {
	switch pathType {
	case PathTypeRoot, PathTypeMemory:
		return base
	case PathTypeFiles:
		return filepath.Join(base, "files")
	case PathTypeAttachments:
		return filepath.Join(base, "attachments")
	case PathTypeLogs:
		return filepath.Join(base, "logs")
	default:
		return base
	}
}

// EnsurePath 确保目录存在并返回路径
func (w *FSWorkspace) EnsurePath(update botcore.RequestSnapshot, pathType PathType) (string, error) {
	path := w.GetPath(update, pathType)
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", fmt.Errorf("create workspace dir: %w", err)
	}
	return path, nil
}

// GetMemoryFile 获取 CLAUDE.md 文件路径
func (w *FSWorkspace) GetMemoryFile(update botcore.RequestSnapshot) string {
	return filepath.Join(w.GetPath(update, PathTypeMemory), "CLAUDE.md")
}

// GetGlobalPath 获取全局共享路径
func (w *FSWorkspace) GetGlobalPath(pathType PathType) string {
	base := filepath.Join(w.baseDir, "global")
	return w.resolvePathType(base, pathType)
}

// EnsureGlobalPath 确保全局目录存在
func (w *FSWorkspace) EnsureGlobalPath(pathType PathType) (string, error) {
	path := w.GetGlobalPath(pathType)
	if err := os.MkdirAll(path, 0755); err != nil {
		return "", fmt.Errorf("create global dir: %w", err)
	}
	return path, nil
}

// ListWorkspaces 列出所有工作空间
func (w *FSWorkspace) ListWorkspaces(ctx context.Context) ([]WorkspaceInfo, error) {
	var workspaces []WorkspaceInfo

	err := filepath.WalkDir(w.baseDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil // 跳过无法访问的目录
		}

		// 查找 CLAUDE.md 文件作为工作空间标识
		if d.Name() == "CLAUDE.md" && !d.IsDir() {
			dir := filepath.Dir(path)
			rel, _ := filepath.Rel(w.baseDir, dir)
			parts := strings.Split(rel, string(os.PathSeparator))

			// 跳过 global 目录
			if len(parts) >= 1 && parts[0] == "global" {
				return nil
			}

			if len(parts) >= 2 {
				info, _ := d.Info()
				ws := WorkspaceInfo{
					Platform:   parts[0],
					ChatID:     parts[1],
					Path:       dir,
					LastAccess: info.ModTime(),
				}

				// 计算目录大小
				ws.SizeBytes = calculateDirSize(dir)

				workspaces = append(workspaces, ws)
			}
		}
		return nil
	})

	return workspaces, err
}

// CleanupOld 清理过期工作空间
func (w *FSWorkspace) CleanupOld(ctx context.Context, olderThan time.Duration) error {
	workspaces, err := w.ListWorkspaces(ctx)
	if err != nil {
		return err
	}

	cutoff := time.Now().Add(-olderThan)
	var lastErr error

	for _, ws := range workspaces {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		if ws.LastAccess.Before(cutoff) {
			if err := os.RemoveAll(ws.Path); err != nil {
				lastErr = err
			}
		}
	}
	return lastErr
}

// sanitizePath 清理路径中的特殊字符，防止路径遍历攻击
func sanitizePath(s string) string {
	if s == "" {
		return "_empty_"
	}
	// 移除或替换不安全的字符
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "..", "_")
	s = strings.ReplaceAll(s, ":", "_")
	s = strings.ReplaceAll(s, "*", "_")
	s = strings.ReplaceAll(s, "?", "_")
	s = strings.ReplaceAll(s, "\"", "_")
	s = strings.ReplaceAll(s, "<", "_")
	s = strings.ReplaceAll(s, ">", "_")
	s = strings.ReplaceAll(s, "|", "_")
	return s
}

// calculateDirSize 计算目录大小
func calculateDirSize(path string) int64 {
	var size int64
	filepath.WalkDir(path, func(_ string, d fs.DirEntry, err error) error {
		if err != nil {
			return nil
		}
		if !d.IsDir() {
			if info, err := d.Info(); err == nil {
				size += info.Size()
			}
		}
		return nil
	})
	return size
}
