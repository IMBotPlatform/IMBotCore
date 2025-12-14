package botcore

// Adapter 将平台原始消息映射为标准 Update。
type Adapter interface {
	Normalize(raw interface{}) (Update, error)
}

// AdapterFunc 允许直接以函数形式实现 Adapter。
type AdapterFunc func(raw interface{}) (Update, error)

// Normalize 实现 Adapter 接口。
func (f AdapterFunc) Normalize(raw interface{}) (Update, error) {
	if f == nil {
		return Update{}, nil
	}
	return f(raw)
}
