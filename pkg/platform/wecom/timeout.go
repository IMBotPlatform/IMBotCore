package wecom

import (
	"os"
	"time"
)

const (
	envBotHTTPTimeout       = "WECOM_BOT_HTTP_TIMEOUT"
	envBotStreamTTL         = "WECOM_BOT_STREAM_TTL"
	envBotStreamWaitTimeout = "WECOM_BOT_STREAM_WAIT_TIMEOUT"
)

// resolveDuration 按参数优先、环境变量次之、默认值兜底的顺序返回超时配置。
func resolveDuration(param time.Duration, envKey string, def time.Duration) time.Duration {
	if param > 0 {
		return param
	}
	if envKey != "" {
		if raw := os.Getenv(envKey); raw != "" {
			if parsed, err := time.ParseDuration(raw); err == nil && parsed > 0 {
				return parsed
			}
		}
	}
	return def
}
