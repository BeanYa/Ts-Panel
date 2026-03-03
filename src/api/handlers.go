package api

import (
	"database/sql"
	"net/http"
	"ts-panel/src/config"
	"ts-panel/src/service"

	"github.com/gin-gonic/gin"
)

// Handler 封装所有 HTTP 处理器的依赖
type Handler struct {
	DB  *sql.DB
	Cfg *config.Config
}

// NewHandler 构造函数
func NewHandler(db *sql.DB, cfg *config.Config) *Handler {
	return &Handler{DB: db, Cfg: cfg}
}

// HealthCheck GET /healthz
func (h *Handler) HealthCheck(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

// Checkout POST /api/instances/checkout
func (h *Handler) Checkout(c *gin.Context) {
	var req service.CheckoutReq
	if err := c.ShouldBindJSON(&req); err != nil {
		ErrResp(c, http.StatusBadRequest, "VALIDATION_ERROR", err.Error())
		return
	}

	resp, err := service.Checkout(c.Request.Context(), h.DB, h.Cfg, req)
	if err != nil {
		status, code := classifyError(err.Error())
		ErrResp(c, status, code, err.Error())
		return
	}
	c.JSON(http.StatusOK, resp)
}

// ListInstances GET /api/instances
func (h *Handler) ListInstances(c *gin.Context) {
	instances, err := service.GetAllInstances(h.DB)
	if err != nil {
		ErrResp(c, http.StatusInternalServerError, "DB_ERROR", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"instances": instances})
}

// GetInstance GET /api/instances/:id
func (h *Handler) GetInstance(c *gin.Context) {
	id := c.Param("id")
	inst, err := service.GetInstanceByID(h.DB, id)
	if err != nil || inst == nil {
		ErrResp(c, http.StatusNotFound, "INSTANCE_NOT_FOUND", "实例不存在")
		return
	}
	c.JSON(http.StatusOK, inst)
}

// StartInstance POST /api/instances/:id/start
func (h *Handler) StartInstance(c *gin.Context) {
	id := c.Param("id")
	if err := service.Start(c.Request.Context(), h.DB, id); err != nil {
		status, code := classifyError(err.Error())
		ErrResp(c, status, code, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": "ok"})
}

// StopInstance POST /api/instances/:id/stop
func (h *Handler) StopInstance(c *gin.Context) {
	id := c.Param("id")
	if err := service.Stop(c.Request.Context(), h.DB, id); err != nil {
		status, code := classifyError(err.Error())
		ErrResp(c, status, code, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": "ok"})
}

// RestartInstance POST /api/instances/:id/restart
func (h *Handler) RestartInstance(c *gin.Context) {
	id := c.Param("id")
	if err := service.Restart(c.Request.Context(), h.DB, id); err != nil {
		status, code := classifyError(err.Error())
		ErrResp(c, status, code, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": "ok"})
}

// RecycleInstance POST /api/instances/:id/recycle
func (h *Handler) RecycleInstance(c *gin.Context) {
	id := c.Param("id")
	var body struct {
		WipeData bool `json:"wipe_data"`
	}
	_ = c.ShouldBindJSON(&body)

	if err := service.Recycle(c.Request.Context(), h.DB, id, body.WipeData); err != nil {
		status, code := classifyError(err.Error())
		ErrResp(c, status, code, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": "ok"})
}

// DeleteInstance DELETE /api/instances/:id?confirm=true
func (h *Handler) DeleteInstance(c *gin.Context) {
	if c.Query("confirm") != "true" {
		ErrResp(c, http.StatusBadRequest, "VALIDATION_ERROR", "必须传入 confirm=true", "危险操作需明确确认")
		return
	}
	id := c.Param("id")
	if err := service.Delete(c.Request.Context(), h.DB, id); err != nil {
		status, code := classifyError(err.Error())
		ErrResp(c, status, code, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": "ok"})
}

// CaptureSecrets POST /api/instances/:id/capture-secrets
func (h *Handler) CaptureSecrets(c *gin.Context) {
	id := c.Param("id")
	inst, err := service.GetInstanceByID(h.DB, id)
	if err != nil || inst == nil {
		ErrResp(c, http.StatusNotFound, "INSTANCE_NOT_FOUND", "实例不存在")
		return
	}

	result, err := service.CaptureSecrets(c.Request.Context(), inst.ContainerName, h.Cfg.LogTail, h.Cfg.SecretsRetry)
	if err != nil {
		ErrResp(c, http.StatusInternalServerError, "DOCKER_ERROR", err.Error())
		return
	}
	if result != nil {
		_ = service.SaveSecrets(h.DB, id, result)
	}
	c.JSON(http.StatusOK, gin.H{"result": "ok", "captured": result != nil})
}

// ApplySlots POST /api/instances/:id/apply-slots
func (h *Handler) ApplySlots(c *gin.Context) {
	id := c.Param("id")
	inst, err := service.GetInstanceByID(h.DB, id)
	if err != nil || inst == nil {
		ErrResp(c, http.StatusNotFound, "INSTANCE_NOT_FOUND", "实例不存在")
		return
	}

	result := service.ApplySlots(c.Request.Context(), h.DB, id, inst.ContainerName, inst.HostQueryPort, inst.Slots, 10)
	if !result.Applied {
		ErrResp(c, http.StatusInternalServerError, "INSTANCE_NOT_READY", result.Error, "容器可能未完全启动，稍后重试")
		return
	}
	c.JSON(http.StatusOK, gin.H{"result": "ok"})
}

// classifyError 根据错误前缀映射 HTTP 状态码和错误码
func classifyError(msg string) (int, string) {
	codes := map[string]int{
		"INSTANCE_NOT_FOUND":  http.StatusNotFound,
		"NO_PORT_AVAILABLE":   http.StatusServiceUnavailable,
		"DOCKER_ERROR":        http.StatusInternalServerError,
		"DB_ERROR":            http.StatusInternalServerError,
		"FS_ERROR":            http.StatusInternalServerError,
		"UNAUTHORIZED":        http.StatusUnauthorized,
		"VALIDATION_ERROR":    http.StatusBadRequest,
		"INSTANCE_NOT_READY":  http.StatusServiceUnavailable,
	}
	for code, status := range codes {
		if len(msg) >= len(code) && msg[:len(code)] == code {
			return status, code
		}
	}
	return http.StatusInternalServerError, "INTERNAL_ERROR"
}
