package service

import (
	"context"
	"database/sql"
	"log"
	"time"
)

// StartExpirationChecker 启动定时任务，每 interval 检查过期实例并自动回收
func StartExpirationChecker(ctx context.Context, sqlDB *sql.DB, interval time.Duration) {
	ticker := time.NewTicker(interval)
	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				log.Println("⏹ 到期检查器停止")
				return
			case <-ticker.C:
				recycleExpired(ctx, sqlDB)
			}
		}
	}()
	log.Printf("⏰ 到期检查器已启动，间隔 %v", interval)
}

// recycleExpired 查找并回收所有已过期的运行中实例
func recycleExpired(ctx context.Context, sqlDB *sql.DB) {
	now := time.Now().UTC().Format(time.RFC3339)
	rows, err := sqlDB.Query(`
		SELECT id FROM instances
		WHERE status = 'running' AND expires_at IS NOT NULL AND expires_at != '' AND expires_at < ?`, now)
	if err != nil {
		log.Printf("⚠ 到期检查查询失败: %v", err)
		return
	}
	defer rows.Close()

	var expiredIDs []string
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err == nil {
			expiredIDs = append(expiredIDs, id)
		}
	}

	for _, id := range expiredIDs {
		log.Printf("♻ 到期回收实例: %s", id)
		if err := Recycle(ctx, sqlDB, id, false); err != nil {
			log.Printf("⚠ 回收实例 %s 失败: %v", id, err)
		}
	}

	if len(expiredIDs) > 0 {
		log.Printf("♻ 本轮回收 %d 个到期实例", len(expiredIDs))
	}
}
