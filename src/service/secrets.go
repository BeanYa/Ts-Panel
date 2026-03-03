package service

import (
	"context"
	"database/sql"
	"fmt"
	"regexp"
	"strings"
	"time"
	"ts-panel/src/docker"
)

var (
	reQueryPassword = regexp.MustCompile(`(?i)serverquery password[:\s]+(\S+)`)
	rePrivilegeKey  = regexp.MustCompile(`(?i)token[=:\s]+([A-Za-z0-9+/=]{20,})`)
)

// CaptureResult secrets 抓取结果
type CaptureResult struct {
	QueryPassword *string
	PrivilegeKey  *string
}

// CaptureSecrets 从容器日志中抓取密钥，带重试
func CaptureSecrets(ctx context.Context, containerName string, logTail, maxRetry int) (*CaptureResult, error) {
	var lastErr error
	for i := 0; i < maxRetry; i++ {
		if i > 0 {
			time.Sleep(1 * time.Second)
		}
		logs, err := docker.Logs(ctx, containerName, logTail)
		if err != nil {
			lastErr = err
			continue
		}
		result := parseSecrets(logs)
		if result.QueryPassword != nil || result.PrivilegeKey != nil {
			return result, nil
		}
		lastErr = fmt.Errorf("日志中未找到密钥（可能容器尚未就绪）")
	}
	return nil, fmt.Errorf("抓取 secrets 失败（重试 %d 次）: %w", maxRetry, lastErr)
}

// parseSecrets 从日志文本中解析密钥
func parseSecrets(logs string) *CaptureResult {
	result := &CaptureResult{}

	for _, line := range strings.Split(logs, "\n") {
		if result.QueryPassword == nil {
			if m := reQueryPassword.FindStringSubmatch(line); len(m) > 1 {
				pass := m[1]
				result.QueryPassword = &pass
			}
		}
		if result.PrivilegeKey == nil {
			if m := rePrivilegeKey.FindStringSubmatch(line); len(m) > 1 {
				key := m[1]
				result.PrivilegeKey = &key
			}
		}
		if result.QueryPassword != nil && result.PrivilegeKey != nil {
			break
		}
	}
	return result
}

// SaveSecrets 持久化到 secrets 表
func SaveSecrets(db *sql.DB, instanceID string, r *CaptureResult) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO secrets (instance_id, serverquery_password, admin_privilege_key, captured_at)
		VALUES (?, ?, ?, ?)
		ON CONFLICT(instance_id) DO UPDATE SET
			serverquery_password = excluded.serverquery_password,
			admin_privilege_key  = excluded.admin_privilege_key,
			captured_at          = excluded.captured_at
	`, instanceID, r.QueryPassword, r.PrivilegeKey, now)
	return err
}
