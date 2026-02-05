package container

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/mount"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

// 输出标记用于健壮的 JSON 解析（需与 agent-runner 脚本保持一致）。
const (
	outputStartMarker = "---IMBOTCORE_OUTPUT_START---"
	outputEndMarker   = "---IMBOTCORE_OUTPUT_END---"
)

// DockerRunner Docker 容器执行器实现。
type DockerRunner struct {
	client *client.Client
	config Config
	mu     sync.Mutex
	active map[string]struct{} // 活跃容器 ID
}

// NewDockerRunner 创建 Docker 执行器。
func NewDockerRunner(cfg Config) (*DockerRunner, error) {
	opts := []client.Opt{client.FromEnv, client.WithAPIVersionNegotiation()}
	if cfg.DockerHost != "" {
		opts = append(opts, client.WithHost(cfg.DockerHost))
	}

	cli, err := client.NewClientWithOpts(opts...)
	if err != nil {
		return nil, fmt.Errorf("create docker client: %w", err)
	}

	if cfg.Image == "" {
		cfg.Image = DefaultConfig().Image
	}

	return &DockerRunner{
		client: cli,
		config: cfg,
		active: make(map[string]struct{}),
	}, nil
}

// Run 在容器中执行 prompt。
func (r *DockerRunner) Run(ctx context.Context, req RunRequest) (*RunResult, error) {
	startTime := time.Now()

	// 确保目录存在
	for _, dir := range []string{req.WorkspaceDir, req.SessionsDir, req.IPCDir} {
		if dir != "" {
			if err := os.MkdirAll(dir, 0755); err != nil {
				return nil, fmt.Errorf("create dir %s: %w", dir, err)
			}
		}
	}

	// 构建挂载配置
	mounts := r.buildMounts(req)

	// 过滤环境变量
	env := r.filterEnvVars(req.EnvVars)

	// 准备输入 JSON
	input := map[string]interface{}{
		"prompt":       req.Prompt,
		"sessionId":    req.SessionID,
		"isNewSession": req.IsNewSession,
		"chatId":       req.ChatID,
		"isMain":       req.IsMain,
	}
	inputJSON, _ := json.Marshal(input)

	// 创建容器
	timeout := req.Timeout
	if timeout == 0 {
		timeout = r.config.DefaultTimeout
	}

	containerConfig := &container.Config{
		Image:        r.config.Image,
		AttachStdin:  true,
		AttachStdout: true,
		AttachStderr: true,
		OpenStdin:    true,
		StdinOnce:    true,
		Tty:          false,
		Env:          env,
		WorkingDir:   "/workspace/group",
	}

	hostConfig := &container.HostConfig{
		Mounts:      mounts,
		NetworkMode: container.NetworkMode(r.config.NetworkMode),
		Resources: container.Resources{
			Memory:   r.config.MemoryLimit,
			CPUQuota: r.config.CPUQuota,
		},
		AutoRemove: true,
	}

	resp, err := r.client.ContainerCreate(ctx, containerConfig, hostConfig, nil, nil, "")
	if err != nil {
		return nil, fmt.Errorf("create container: %w", err)
	}
	containerID := resp.ID

	r.mu.Lock()
	r.active[containerID] = struct{}{}
	r.mu.Unlock()
	defer func() {
		r.mu.Lock()
		delete(r.active, containerID)
		r.mu.Unlock()
	}()

	// Attach stdin/stdout/stderr
	attachResp, err := r.client.ContainerAttach(ctx, containerID, container.AttachOptions{
		Stream: true,
		Stdin:  true,
		Stdout: true,
		Stderr: true,
	})
	if err != nil {
		return nil, fmt.Errorf("attach container: %w", err)
	}
	defer attachResp.Close()

	// 启动容器
	if err := r.client.ContainerStart(ctx, containerID, container.StartOptions{}); err != nil {
		return nil, fmt.Errorf("start container: %w", err)
	}

	// 写入输入
	go func() {
		defer attachResp.CloseWrite()
		attachResp.Conn.Write(inputJSON)
	}()

	// 设置超时
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	// 读取输出
	var stdout, stderr bytes.Buffer
	outputDone := make(chan error, 1)
	go func() {
		_, err := stdcopy.StdCopy(&stdout, &stderr, attachResp.Reader)
		outputDone <- err
	}()

	// 等待容器结束或超时
	statusCh, errCh := r.client.ContainerWait(ctx, containerID, container.WaitConditionNotRunning)
	var exitCode int64

	select {
	case err := <-errCh:
		if err != nil {
			// 超时，强制停止
			stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
			defer stopCancel()
			r.client.ContainerStop(stopCtx, containerID, container.StopOptions{})
			return &RunResult{
				Status:   "error",
				Error:    fmt.Sprintf("container wait error: %v", err),
				Duration: time.Since(startTime),
			}, nil
		}
	case status := <-statusCh:
		exitCode = status.StatusCode
	case <-ctx.Done():
		// 超时
		stopCtx, stopCancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer stopCancel()
		r.client.ContainerStop(stopCtx, containerID, container.StopOptions{})
		return &RunResult{
			Status:   "error",
			Error:    fmt.Sprintf("container timeout after %v", timeout),
			Duration: time.Since(startTime),
			ExitCode: -1,
		}, nil
	}

	<-outputDone

	// 解析输出
	result := r.parseOutput(stdout.String(), stderr.String(), exitCode, startTime)
	return result, nil
}

// buildMounts 构建容器挂载配置。
func (r *DockerRunner) buildMounts(req RunRequest) []mount.Mount {
	mounts := []mount.Mount{}

	// 工作空间目录
	if req.WorkspaceDir != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: req.WorkspaceDir,
			Target: "/workspace/group",
		})
	}

	// Claude sessions 目录
	if req.SessionsDir != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: req.SessionsDir,
			Target: "/home/node/.claude",
		})
	}

	// IPC 目录
	if req.IPCDir != "" {
		mounts = append(mounts, mount.Mount{
			Type:   mount.TypeBind,
			Source: req.IPCDir,
			Target: "/workspace/ipc",
		})
	}

	// 全局记忆目录（只读）
	if req.GlobalDir != "" {
		mounts = append(mounts, mount.Mount{
			Type:     mount.TypeBind,
			Source:   req.GlobalDir,
			Target:   "/workspace/global",
			ReadOnly: true,
		})
	}

	return mounts
}

// filterEnvVars 过滤环境变量，只保留允许的变量。
func (r *DockerRunner) filterEnvVars(envVars map[string]string) []string {
	allowed := r.config.AllowedEnvVars
	if len(allowed) == 0 {
		allowed = DefaultConfig().AllowedEnvVars
	}

	allowedSet := make(map[string]bool)
	for _, v := range allowed {
		allowedSet[v] = true
	}

	result := []string{}

	// 从请求中复制允许的变量
	for k, v := range envVars {
		if allowedSet[k] {
			result = append(result, fmt.Sprintf("%s=%s", k, v))
		}
	}

	// 从当前环境中补充
	for _, k := range allowed {
		if _, exists := envVars[k]; !exists {
			if v := os.Getenv(k); v != "" {
				result = append(result, fmt.Sprintf("%s=%s", k, v))
			}
		}
	}

	return result
}

// parseOutput 解析容器输出。
func (r *DockerRunner) parseOutput(stdout, stderr string, exitCode int64, startTime time.Time) *RunResult {
	result := &RunResult{
		Duration: time.Since(startTime),
		ExitCode: int(exitCode),
	}

	if exitCode != 0 {
		result.Status = "error"
		result.Error = fmt.Sprintf("container exited with code %d: %s", exitCode, lastNChars(stderr, 200))
		return result
	}

	// 尝试从标记中提取 JSON
	startIdx := strings.Index(stdout, outputStartMarker)
	endIdx := strings.Index(stdout, outputEndMarker)

	var jsonStr string
	if startIdx != -1 && endIdx != -1 && endIdx > startIdx {
		jsonStr = strings.TrimSpace(stdout[startIdx+len(outputStartMarker) : endIdx])
	} else {
		// 回退：最后一行
		lines := strings.Split(strings.TrimSpace(stdout), "\n")
		if len(lines) > 0 {
			jsonStr = lines[len(lines)-1]
		}
	}

	if jsonStr != "" {
		var output struct {
			Status       string `json:"status"`
			Result       string `json:"result"`
			NewSessionID string `json:"newSessionId"`
			Error        string `json:"error"`
		}
		if err := json.Unmarshal([]byte(jsonStr), &output); err == nil {
			result.Status = output.Status
			result.Output = output.Result
			result.NewSessionID = output.NewSessionID
			if output.Error != "" {
				result.Error = output.Error
			}
			return result
		}
	}

	// JSON 解析失败，返回原始输出
	result.Status = "success"
	result.Output = stdout
	return result
}

// Stop 停止指定容器。
func (r *DockerRunner) Stop(containerID string) error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return r.client.ContainerStop(ctx, containerID, container.StopOptions{})
}

// Cleanup 清理过期资源。
func (r *DockerRunner) Cleanup(ctx context.Context) error {
	// 目前不需要特殊清理，AutoRemove=true 会自动删除容器
	return nil
}

// Close 关闭执行器。
func (r *DockerRunner) Close() error {
	return r.client.Close()
}

// lastNChars 返回字符串最后 n 个字符。
func lastNChars(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[len(s)-n:]
}

// WriteEnvFile 将环境变量写入文件（用于容器内读取）。
func WriteEnvFile(dir string, envVars map[string]string, allowedVars []string) (string, error) {
	if err := os.MkdirAll(dir, 0755); err != nil {
		return "", err
	}

	allowedSet := make(map[string]bool)
	for _, v := range allowedVars {
		allowedSet[v] = true
	}

	var lines []string
	for k, v := range envVars {
		if allowedSet[k] {
			lines = append(lines, fmt.Sprintf("%s=%s", k, v))
		}
	}

	envFile := filepath.Join(dir, "env")
	if len(lines) > 0 {
		if err := os.WriteFile(envFile, []byte(strings.Join(lines, "\n")+"\n"), 0600); err != nil {
			return "", err
		}
	}

	return envFile, nil
}
