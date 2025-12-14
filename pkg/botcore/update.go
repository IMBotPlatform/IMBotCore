package botcore

// Update 描述任意聊天/机器人平台上的标准化事件。
type Update struct {
	ID       string            // 平台内的唯一消息或事件 ID
	SenderID string            // 触发用户标识
	ChatID   string            // 会话 ID（群、私聊等）
	ChatType string            // 会话类型，示例：single/chatroom
	Text     string            // 主要文本内容（若适用）
	Raw      interface{}       // 平台原始结构引用，便于 Handler 深度使用
	Metadata map[string]string // 扩展键值，如语言、平台等
}

// CloneMetadata 返回一份 Metadata 拷贝，防止 Handler 意外修改底层数据。
func (u Update) CloneMetadata() map[string]string {
	if len(u.Metadata) == 0 {
		return nil
	}
	out := make(map[string]string, len(u.Metadata))
	for k, v := range u.Metadata {
		out[k] = v
	}
	return out
}
