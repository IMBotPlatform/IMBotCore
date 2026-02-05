package container

import (
	"os"
	"path/filepath"
	"strings"
)

// DefaultBlockedPatterns 默认禁止挂载的路径模式。
var DefaultBlockedPatterns = []string{
	".ssh",
	".gnupg",
	".gpg",
	".aws",
	".azure",
	".gcloud",
	".kube",
	".docker",
	"credentials",
	".env",
	".netrc",
	".npmrc",
	".pypirc",
	"id_rsa",
	"id_ed25519",
	"private_key",
	".secret",
}

// MountValidator 挂载验证器。
type MountValidator struct {
	blockedPatterns []string
}

// NewMountValidator 创建挂载验证器。
func NewMountValidator(additionalBlocked []string) *MountValidator {
	patterns := make([]string, len(DefaultBlockedPatterns))
	copy(patterns, DefaultBlockedPatterns)
	patterns = append(patterns, additionalBlocked...)
	return &MountValidator{blockedPatterns: patterns}
}

// ValidateResult 验证结果。
type ValidateResult struct {
	Allowed  bool   // 是否允许
	Reason   string // 原因
	RealPath string // 解析后的真实路径
}

// Validate 验证挂载路径。
func (v *MountValidator) Validate(hostPath string) ValidateResult {
	// 展开 ~ 为 home 目录
	if strings.HasPrefix(hostPath, "~/") {
		home, _ := os.UserHomeDir()
		hostPath = filepath.Join(home, hostPath[2:])
	}

	// 解析真实路径
	realPath, err := filepath.EvalSymlinks(hostPath)
	if err != nil {
		// 路径不存在
		return ValidateResult{
			Allowed: false,
			Reason:  "path does not exist: " + hostPath,
		}
	}

	// 检查是否匹配禁止模式
	pathParts := strings.Split(realPath, string(os.PathSeparator))
	for _, pattern := range v.blockedPatterns {
		for _, part := range pathParts {
			if part == pattern || strings.Contains(part, pattern) {
				return ValidateResult{
					Allowed: false,
					Reason:  "path matches blocked pattern: " + pattern,
				}
			}
		}
	}

	return ValidateResult{
		Allowed:  true,
		Reason:   "allowed",
		RealPath: realPath,
	}
}

// ValidateMounts 批量验证挂载路径。
// 返回验证通过的挂载列表和被拒绝的列表。
func (v *MountValidator) ValidateMounts(mounts []VolumeMount) (allowed []VolumeMount, rejected []VolumeMount) {
	for _, m := range mounts {
		result := v.Validate(m.HostPath)
		if result.Allowed {
			allowed = append(allowed, VolumeMount{
				HostPath:      result.RealPath,
				ContainerPath: m.ContainerPath,
				ReadOnly:      m.ReadOnly,
			})
		} else {
			rejected = append(rejected, m)
		}
	}
	return
}

// SanitizePath 清理路径中的特殊字符。
func SanitizePath(s string) string {
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "..", "_")
	s = strings.ReplaceAll(s, ":", "_")
	return s
}
