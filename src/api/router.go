package api

import (
	"database/sql"
	"ts-panel/src/config"

	"github.com/gin-gonic/gin"
)

// SetupRouter 注册所有路由
func SetupRouter(db *sql.DB, cfg *config.Config) *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())

	h := NewHandler(db, cfg)

	// 健康检查（无需鉴权）
	r.GET("/healthz", h.HealthCheck)

	// API 路由组（需要 Token 鉴权）
	api := r.Group("/api")
	api.Use(AuthMiddleware(cfg.AdminToken))
	{
		// 实例核心操作
		instances := api.Group("/instances")
		{
			instances.POST("/checkout", h.Checkout)
			instances.POST("/restore-checkout", h.RestoreCheckout)
			instances.GET("", h.ListInstances)
			instances.GET("/:id", h.GetInstance)

			// 生命周期
			instances.POST("/:id/start", h.StartInstance)
			instances.POST("/:id/stop", h.StopInstance)
			instances.POST("/:id/restart", h.RestartInstance)
			instances.POST("/:id/recycle", h.RecycleInstance)
			instances.DELETE("/:id", h.DeleteInstance)

			// secrets / slots / logs
			instances.POST("/:id/capture-secrets", h.CaptureSecrets)
			instances.POST("/:id/apply-slots", h.ApplySlots)
			instances.GET("/:id/logs", h.GetContainerLogs)

			// 备份 / 恢复
			instances.GET("/:id/backup", h.BackupInstance)
			instances.POST("/:id/restore", h.RestoreInstance)
		}
	}

	return r
}
