package service

import (
	"context"
	"database/sql"
	"fmt"
	"time"
	"ts-panel/src/tsquery"
)

// ApplySlotsResult 返回结果
type ApplySlotsResult struct {
	Applied bool
	Error   string
}

// ApplySlots 通过 ServerQuery 设置 maxclients，带重试
func ApplySlots(ctx context.Context, db *sql.DB, instanceID, containerName string, queryPort, slots, maxRetry int) *ApplySlotsResult {
	var lastErr error
	for i := 0; i < maxRetry; i++ {
		if i > 0 {
			time.Sleep(1 * time.Second)
		}

		// 获取 serverquery_password
		var password sql.NullString
		if err := db.QueryRow(`SELECT serverquery_password FROM secrets WHERE instance_id = ?`, instanceID).Scan(&password); err != nil || !password.Valid {
			lastErr = fmt.Errorf("未找到 serverquery_password")
			continue
		}

		addr := fmt.Sprintf("127.0.0.1:%d", queryPort)
		if err := doApplySlots(addr, password.String, slots); err != nil {
			lastErr = err
			continue
		}

		// 成功，更新 slots_applied
		_, _ = db.Exec(`UPDATE instances SET slots_applied=1, updated_at=? WHERE id=?`,
			time.Now().UTC().Format(time.RFC3339), instanceID)
		return &ApplySlotsResult{Applied: true}
	}

	errMsg := ""
	if lastErr != nil {
		errMsg = lastErr.Error()
	}
	return &ApplySlotsResult{Applied: false, Error: errMsg}
}

func doApplySlots(addr, password string, slots int) error {
	c, err := tsquery.Dial(addr)
	if err != nil {
		return err
	}
	defer c.Close()

	if err := c.Login("serveradmin", password); err != nil {
		return fmt.Errorf("Login 失败: %w", err)
	}

	// 先尝试 use sid=1，失败则 serverlist 找第一个
	if err := c.Use(1); err != nil {
		sid, err2 := c.ServerList()
		if err2 != nil {
			return fmt.Errorf("ServerList 失败: %w", err2)
		}
		if err := c.Use(sid); err != nil {
			return fmt.Errorf("Use sid=%d 失败: %w", sid, err)
		}
	}

	return c.SetMaxClients(slots)
}
