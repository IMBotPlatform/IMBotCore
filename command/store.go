package command

import "sync"

// MemoryStore 提供简单的基于内存的上下文存储实现。
// 仅用于命令执行期的上下文键值（非聊天历史）；进程重启即丢失。
type MemoryStore struct {
	mu   sync.RWMutex
	data map[string]ContextValues
}

// NewMemoryStore 创建内存存储实例。
func NewMemoryStore() *MemoryStore {
	return &MemoryStore{data: make(map[string]ContextValues)}
}

// Load 返回指定 key 的上下文副本。
func (s *MemoryStore) Load(key string) (ContextValues, error) {
	if s == nil || key == "" {
		return nil, nil
	}
	s.mu.RLock()
	defer s.mu.RUnlock()
	if val, ok := s.data[key]; ok {
		return cloneValues(val), nil
	}
	return nil, nil
}

// Save 合并并存储上下文增量。
func (s *MemoryStore) Save(key string, values ContextValues) error {
	if s == nil || key == "" || len(values) == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	current := s.data[key]
	merged := cloneValues(current)
	if merged == nil {
		merged = ContextValues{}
	}
	// 合并语义：同名键按最新值覆盖；caller 需自行避免空 key。
	for k, v := range values {
		merged[k] = v
	}
	s.data[key] = merged
	return nil
}

// cloneValues 复制上下文字典，避免共享引用。
func cloneValues(src ContextValues) ContextValues {
	if len(src) == 0 {
		return nil
	}
	dst := make(ContextValues, len(src))
	for k, v := range src {
		dst[k] = v
	}
	return dst
}
