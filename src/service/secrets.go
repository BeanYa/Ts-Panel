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
	// Server Query Admin Account 区块
	reLoginName = regexp.MustCompile(`(?i)loginname\s*=\s*"([^"]+)"`)
	reAdminPass = regexp.MustCompile(`(?i)loginname\s*=\s*"[^"]+",\s*password\s*=\s*"([^"]+)"`)
	reAPIKey    = regexp.MustCompile(`(?i)apikey\s*=\s*"([^"]+)"`)

	// Admin privilege token (首次创建服务器时的 token=xxx 行)
	rePrivilegeKey = regexp.MustCompile(`(?i)token\s*=\s*([A-Za-z0-9+/=]{20,})`)

	// 旧版 ServerQuery password 行（兜底）
	reQueryPassword = regexp.MustCompile(`(?i)serverquery password[:\s]+(\S+)`)
)

// CaptureResult secrets 抓取结果
type CaptureResult struct {
	// Server Query Admin Account（管理员账号区块）
	LoginName     *string `json:"login_name,omitempty"`
	AdminPassword *string `json:"admin_password,omitempty"`
	APIKey        *string `json:"api_key,omitempty"`

	// Admin Privilege Token（首次服务器创建 token）
	PrivilegeKey *string `json:"privilege_key,omitempty"`

	// ServerQuery Password（旧版兜底）
	QueryPassword *string `json:"query_password,omitempty"`
}

// hasAnySecret 判断是否已找到至少一个关键秘密
func (r *CaptureResult) hasAnySecret() bool {
	return r.AdminPassword != nil || r.PrivilegeKey != nil || r.QueryPassword != nil
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
		if result.hasAnySecret() {
			return result, nil
		}
		lastErr = fmt.Errorf("日志中未找到密钥（可能容器尚未就绪）")
	}
	return nil, fmt.Errorf("抓取 secrets 失败（重试 %d 次）: %w", maxRetry, lastErr)
}

// parseSecrets 从日志文本中解析所有密钥
func parseSecrets(logs string) *CaptureResult {
	result := &CaptureResult{}

	for _, line := range strings.Split(logs, "\n") {
		if result.LoginName == nil {
			if m := reLoginName.FindStringSubmatch(line); len(m) > 1 {
				v := m[1]
				result.LoginName = &v
			}
		}
		if result.AdminPassword == nil {
			if m := reAdminPass.FindStringSubmatch(line); len(m) > 1 {
				v := m[1]
				result.AdminPassword = &v
			}
		}
		if result.APIKey == nil {
			if m := reAPIKey.FindStringSubmatch(line); len(m) > 1 {
				v := m[1]
				result.APIKey = &v
			}
		}
		if result.PrivilegeKey == nil {
			if m := rePrivilegeKey.FindStringSubmatch(line); len(m) > 1 {
				v := m[1]
				result.PrivilegeKey = &v
			}
		}
		if result.QueryPassword == nil {
			if m := reQueryPassword.FindStringSubmatch(line); len(m) > 1 {
				v := m[1]
				result.QueryPassword = &v
			}
		}
	}

	// 特殊逻辑：如果抓到了 serveradmin 的 password，但没抓到 query_password，则同步
	if result.QueryPassword == nil && result.AdminPassword != nil && result.LoginName != nil && *result.LoginName == "serveradmin" {
		result.QueryPassword = result.AdminPassword
	}

	return result
}

// SaveSecrets 持久化到 secrets 表
func SaveSecrets(db *sql.DB, instanceID string, r *CaptureResult) error {
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := db.Exec(`
		INSERT INTO secrets
		(instance_id, login_name, admin_password, api_key, serverquery_password, admin_privilege_key, captured_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(instance_id) DO UPDATE SET
			login_name           = excluded.login_name,
			admin_password       = excluded.admin_password,
			api_key              = excluded.api_key,
			serverquery_password = excluded.serverquery_password,
			admin_privilege_key  = excluded.admin_privilege_key,
			captured_at          = excluded.captured_at
	`, instanceID, r.LoginName, r.AdminPassword, r.APIKey, r.QueryPassword, r.PrivilegeKey, now)
	return err
}
