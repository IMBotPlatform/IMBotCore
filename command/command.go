package command

import "github.com/spf13/cobra"

// CommandFactory 定义创建 Cobra 命令树的工厂函数类型。
// 在 HTTP 服务中，每个请求必须拥有独立的命令对象实例，以避免 Flag 解析的并发冲突。
type CommandFactory func() *cobra.Command