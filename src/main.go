package main

import (
	"context"
	"log"
	"time"
	"ts-panel/src/api"
	"ts-panel/src/config"
	"ts-panel/src/db"
	"ts-panel/src/service"
)

func main() {
	// 1. 加载配置
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("配置加载失败: %v", err)
	}

	// 2. 初始化数据库
	sqlDB, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("数据库初始化失败: %v", err)
	}
	defer sqlDB.Close()

	// 3. 执行数据库迁移
	if err := db.RunMigrations(sqlDB); err != nil {
		log.Fatalf("数据库迁移失败: %v", err)
	}

	log.Printf("📡 ts-panel 启动，Public IP: %s, HTTP Port: %s", cfg.PublicIP, cfg.HTTPPort)
	log.Printf("🔌 UDP 端口池: %d-%d | Query 端口池: %d-%d",
		cfg.PortMin, cfg.PortMax, cfg.QueryPortMin, cfg.QueryPortMax)

	// 4. 启动到期检查器（每小时）
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	service.StartExpirationChecker(ctx, sqlDB, 1*time.Hour)

	// 5. 启动 Gin
	r := api.SetupRouter(sqlDB, cfg)
	if err := r.Run(":" + cfg.HTTPPort); err != nil {
		log.Fatalf("服务器启动失败: %v", err)
	}
}
