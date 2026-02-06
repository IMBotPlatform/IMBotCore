package botcore

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"
)

const envSaveAttachTimeout = "WECOM_BOT_SAVE_ATTACH_TIMEOUT"

// ChatType 描述会话类型枚举。
type ChatType string

const (
	ChatTypeSingle   ChatType = "single"   // 单聊
	ChatTypeChatroom ChatType = "chatroom" // 群聊
)

// RequestSnapshot 描述首包请求的标准化快照。
type RequestSnapshot struct {
	ID       string   // 平台内的唯一消息、事件或流会话 ID
	SenderID string   // 触发用户标识
	ChatID   string   // 会话 ID（群、私聊等）
	ChatType ChatType // 会话类型，示例：single/chatroom（企业微信为 single/group，内部映射为 chatroom）

	Text        string            // 主要文本内容（若适用）
	Attachments []Attachment      // 标准化附件列表（图片/文件等）
	Raw         any               // 平台原始结构引用，便于 Pipeline 深度使用
	ResponseURL string            // 主动回复 URL（部分平台返回）
	Metadata    map[string]string // 扩展键值，如语言、平台等
}

// AttachmentType 描述附件类型。
type AttachmentType string

const (
	// AttachmentTypeImage 表示图片附件。
	AttachmentTypeImage AttachmentType = "image"
	// AttachmentTypeFile 表示文件附件。
	AttachmentTypeFile AttachmentType = "file"
)

// Attachment 描述平台无关的附件信息。
type Attachment struct {
	Type AttachmentType // 附件类型: image/file
	URL  string         // 可下载的资源地址（当 Data 为空时使用）
	// Data 存储已解密/已下载的原始字节数据。
	// 当此字段非空时，SaveAttachments 将直接使用此数据而不是下载 URL。
	// 由平台协议层（如 wecom）自动填充已解密的附件数据。
	Data []byte
}

// SavedAttachment 表示附件保存结果。
type SavedAttachment struct {
	Attachment Attachment // 原始附件信息
	Path       string     // 保存后的本地路径
	Err        error      // 单个附件的错误（若有）
}

// SaveAttachments 下载并保存所有附件到指定目录。
// Parameters:
//   - dir: 保存目录（不存在会创建）
//
// Returns:
//   - []SavedAttachment: 每个附件的保存结果
//   - error: 只要有任意附件失败则返回非空错误
func (r RequestSnapshot) SaveAttachments(dir string) ([]SavedAttachment, error) {
	if strings.TrimSpace(dir) == "" {
		return nil, errors.New("save dir is empty")
	}

	// 关键步骤：确保目标目录存在。
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, fmt.Errorf("create dir: %w", err)
	}

	if len(r.Attachments) == 0 {
		return []SavedAttachment{}, nil
	}

	clientTimeout := resolveDurationFromEnv(envSaveAttachTimeout, 2*time.Minute)
	client := &http.Client{Timeout: clientTimeout}
	results := make([]SavedAttachment, 0, len(r.Attachments))
	var hasError bool

	for i, att := range r.Attachments {
		result := SavedAttachment{Attachment: att}

		// 关键步骤：优先使用 Data 字段（协议层已解密的数据），仅当 Data 为空时才下载 URL。
		if len(att.Data) == 0 && strings.TrimSpace(att.URL) == "" {
			result.Err = errors.New("attachment has no data and no url")
			results = append(results, result)
			hasError = true
			continue
		}

		filename := deriveAttachmentFileName(att.URL, att.Type, i)
		targetPath, err := uniqueAttachmentPath(dir, filename)
		if err != nil {
			result.Err = err
			results = append(results, result)
			hasError = true
			continue
		}

		// 关键步骤：优先使用已解密的 Data，若无则下载 URL。
		if len(att.Data) > 0 {
			// 直接写入已解密数据
			if err := os.WriteFile(targetPath, att.Data, 0o644); err != nil {
				result.Err = fmt.Errorf("write data: %w", err)
				results = append(results, result)
				hasError = true
				continue
			}
		} else {
			// 下载并落盘
			if err := downloadAttachment(client, att.URL, targetPath); err != nil {
				result.Err = err
				results = append(results, result)
				hasError = true
				continue
			}
		}

		result.Path = targetPath
		results = append(results, result)
	}

	if hasError {
		return results, errors.New("save attachments: some downloads failed")
	}
	return results, nil
}

// resolveDurationFromEnv 读取环境变量中的超时配置。
func resolveDurationFromEnv(envKey string, def time.Duration) time.Duration {
	if strings.TrimSpace(envKey) == "" {
		return def
	}
	raw := strings.TrimSpace(os.Getenv(envKey))
	if raw == "" {
		return def
	}
	parsed, err := time.ParseDuration(raw)
	if err != nil || parsed <= 0 {
		return def
	}
	return parsed
}

// deriveAttachmentFileName 根据 URL 推导文件名。
func deriveAttachmentFileName(rawURL string, attType AttachmentType, index int) string {
	filename := ""
	if parsed, err := url.Parse(rawURL); err == nil {
		filename = path.Base(parsed.Path)
	}

	filename = strings.TrimSpace(filename)
	if filename == "" || filename == "." || filename == "/" {
		filename = fmt.Sprintf("%s_%d", attType, index)
	}

	// 关键步骤：避免路径分隔符污染文件名。
	filename = strings.ReplaceAll(filename, "/", "_")
	filename = strings.ReplaceAll(filename, "\\", "_")

	return filename
}

// uniqueAttachmentPath 在目标目录内生成不冲突的文件路径。
func uniqueAttachmentPath(dir, filename string) (string, error) {
	if strings.TrimSpace(filename) == "" {
		return "", errors.New("filename is empty")
	}

	target := filepath.Join(dir, filename)
	if _, err := os.Stat(target); err != nil {
		if os.IsNotExist(err) {
			return target, nil
		}
		return "", fmt.Errorf("stat file: %w", err)
	}

	ext := filepath.Ext(filename)
	base := strings.TrimSuffix(filename, ext)
	for i := 1; i < 10000; i++ {
		candidate := fmt.Sprintf("%s_%d%s", base, i, ext)
		target = filepath.Join(dir, candidate)
		if _, err := os.Stat(target); err != nil {
			if os.IsNotExist(err) {
				return target, nil
			}
			return "", fmt.Errorf("stat file: %w", err)
		}
	}

	return "", fmt.Errorf("cannot allocate unique filename for %s", filename)
}

// downloadAttachment 下载远程资源并写入本地文件。
func downloadAttachment(client *http.Client, rawURL, targetPath string) error {
	req, err := http.NewRequest(http.MethodGet, rawURL, nil)
	if err != nil {
		return fmt.Errorf("create request: %w", err)
	}

	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("download attachment: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("download attachment: status=%d", resp.StatusCode)
	}

	out, err := os.Create(targetPath)
	if err != nil {
		return fmt.Errorf("create file: %w", err)
	}
	defer out.Close()

	if _, err := io.Copy(out, resp.Body); err != nil {
		_ = os.Remove(targetPath)
		return fmt.Errorf("write file: %w", err)
	}

	return nil
}
