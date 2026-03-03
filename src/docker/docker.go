package docker

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"time"
)

const defaultTimeout = 30 * time.Second

// ContainerParams 创建容器所需参数
type ContainerParams struct {
	Name       string
	InstanceID string
	CustomerID string
	DataPath   string
	UDPPort    int
	QueryPort  int
	CPU        string
	Memory     string
	Pids       int
	Slots      int
}

// CreateAndStart 创建并启动容器，失败时自动清理并重试
func CreateAndStart(ctx context.Context, params ContainerParams, maxRetry int) error {
	var lastErr error
	for i := 0; i <= maxRetry; i++ {
		if i > 0 {
			// 重试前先清理
			_ = Remove(ctx, params.Name)
			time.Sleep(500 * time.Millisecond)
		}
		if err := create(ctx, params); err != nil {
			lastErr = err
			continue
		}
		if err := Start(ctx, params.Name); err != nil {
			lastErr = err
			_ = Remove(ctx, params.Name)
			continue
		}
		return nil
	}
	return fmt.Errorf("容器 %s 创建失败（重试 %d 次）: %w", params.Name, maxRetry, lastErr)
}

// create 执行 docker create
func create(ctx context.Context, p ContainerParams) error {
	args := []string{
		"create",
		"--name", p.Name,
		"--restart", "unless-stopped",
		"--label", "managed_by=ts-panel",
		"--label", fmt.Sprintf("instance_id=%s", p.InstanceID),
		"--label", fmt.Sprintf("customer_id=%s", p.CustomerID),
		"--mount", fmt.Sprintf("type=bind,src=%s,dst=/var/ts3server", p.DataPath),
		"-e", "TS3SERVER_LICENSE=accept",
		"--cpus", p.CPU,
		"--memory", p.Memory,
		"--pids-limit", fmt.Sprintf("%d", p.Pids),
		"--log-opt", "max-size=10m",
		"--log-opt", "max-file=3",
		"-p", fmt.Sprintf("%d:9987/udp", p.UDPPort),
		"-p", fmt.Sprintf("127.0.0.1:%d:10011/tcp", p.QueryPort),
		"teamspeak:latest",
	}
	return run(ctx, args...)
}

// Start 启动容器
func Start(ctx context.Context, name string) error {
	return run(ctx, "start", name)
}

// Stop 停止容器
func Stop(ctx context.Context, name string) error {
	return run(ctx, "stop", name)
}

// Restart 重启容器
func Restart(ctx context.Context, name string) error {
	return run(ctx, "restart", name)
}

// Remove 强制删除容器
func Remove(ctx context.Context, name string) error {
	return run(ctx, "rm", "-f", name)
}

// Logs 获取容器日志（最后 N 行）
func Logs(ctx context.Context, name string, tail int) (string, error) {
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(tctx, "docker", "logs", "--tail", fmt.Sprintf("%d", tail), name)
	var out bytes.Buffer
	var errOut bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &errOut // docker logs 输出到 stderr

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("docker logs 失败: %w, stderr: %s", err, errOut.String())
	}
	// docker logs 可能同时写 stdout 和 stderr
	return out.String() + errOut.String(), nil
}

// Inspect 检查容器是否存在及状态
func Inspect(ctx context.Context, name string) (string, error) {
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(tctx, "docker", "inspect", "--format", "{{.State.Status}}", name)
	var out bytes.Buffer
	cmd.Stdout = &out

	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("容器 %s 不存在或检查失败", name)
	}
	return strings.TrimSpace(out.String()), nil
}

// run 执行 docker 命令
func run(ctx context.Context, args ...string) error {
	tctx, cancel := context.WithTimeout(ctx, defaultTimeout)
	defer cancel()

	cmd := exec.CommandContext(tctx, "docker", args...)
	var errOut bytes.Buffer
	cmd.Stderr = &errOut

	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker %s 失败: %w, stderr: %s", args[0], err, errOut.String())
	}
	return nil
}
