package command

import "errors"

// 定义命令解析与分发阶段的通用错误，便于统一处理提示文案。
var (
	// ErrCommandNotFound 表示输入命令在注册表中不存在。
	ErrCommandNotFound = errors.New("command not found")
	// ErrCommandRequired 表示未提供任何命令关键字。
	ErrCommandRequired = errors.New("command required")
)
