package api

import (
	"archive/tar"
	"compress/gzip"
	"database/sql"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
	"ts-panel/src/config"
	"ts-panel/src/docker"
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
		"INSTANCE_NOT_FOUND": http.StatusNotFound,
		"NO_PORT_AVAILABLE":  http.StatusServiceUnavailable,
		"DOCKER_ERROR":       http.StatusInternalServerError,
		"DB_ERROR":           http.StatusInternalServerError,
		"FS_ERROR":           http.StatusInternalServerError,
		"UNAUTHORIZED":       http.StatusUnauthorized,
		"VALIDATION_ERROR":   http.StatusBadRequest,
		"INSTANCE_NOT_READY": http.StatusServiceUnavailable,
	}
	for code, status := range codes {
		if len(msg) >= len(code) && msg[:len(code)] == code {
			return status, code
		}
	}
	return http.StatusInternalServerError, "INTERNAL_ERROR"
}

// GetContainerLogs GET /api/instances/:id/logs
func (h *Handler) GetContainerLogs(c *gin.Context) {
	id := c.Param("id")
	inst, err := service.GetInstanceByID(h.DB, id)
	if err != nil || inst == nil {
		ErrResp(c, http.StatusNotFound, "INSTANCE_NOT_FOUND", "实例不存在")
		return
	}

	tail := 100
	if t := c.Query("tail"); t != "" {
		if n, err := strconv.Atoi(t); err == nil && n > 0 {
			tail = n
		}
	}

	logs, err := docker.Logs(c.Request.Context(), inst.ContainerName, tail)
	if err != nil {
		ErrResp(c, http.StatusInternalServerError, "DOCKER_ERROR", err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"logs": logs})
}

// BackupInstance GET /api/instances/:id/backup
func (h *Handler) BackupInstance(c *gin.Context) {
	id := c.Param("id")
	inst, err := service.GetInstanceByID(h.DB, id)
	if err != nil || inst == nil {
		ErrResp(c, http.StatusNotFound, "INSTANCE_NOT_FOUND", "实例不存在")
		return
	}

	if inst.DataPath == "" {
		ErrResp(c, http.StatusBadRequest, "VALIDATION_ERROR", "实例无数据目录")
		return
	}
	if _, err := os.Stat(inst.DataPath); os.IsNotExist(err) {
		ErrResp(c, http.StatusBadRequest, "FS_ERROR", "数据目录不存在")
		return
	}

	filename := fmt.Sprintf("backup-%s-%s.tar.gz", inst.ContainerName, time.Now().Format("20060102150405"))
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	c.Header("Content-Type", "application/gzip")

	gw := gzip.NewWriter(c.Writer)
	defer gw.Close()
	tw := tar.NewWriter(gw)
	defer tw.Close()

	_ = filepath.Walk(inst.DataPath, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		relPath, _ := filepath.Rel(inst.DataPath, path)
		if relPath == "." {
			return nil
		}
		header, err := tar.FileInfoHeader(info, "")
		if err != nil {
			return err
		}
		header.Name = filepath.ToSlash(relPath)
		if err := tw.WriteHeader(header); err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		f, err := os.Open(path)
		if err != nil {
			return err
		}
		defer f.Close()
		_, _ = io.Copy(tw, f)
		return nil
	})
}

// RestoreInstance POST /api/instances/:id/restore
func (h *Handler) RestoreInstance(c *gin.Context) {
	id := c.Param("id")
	inst, err := service.GetInstanceByID(h.DB, id)
	if err != nil || inst == nil {
		ErrResp(c, http.StatusNotFound, "INSTANCE_NOT_FOUND", "实例不存在")
		return
	}

	file, _, err := c.Request.FormFile("file")
	if err != nil {
		ErrResp(c, http.StatusBadRequest, "VALIDATION_ERROR", "请上传 .tar.gz 备份文件")
		return
	}
	defer file.Close()

	// 停止容器
	_ = docker.Stop(c.Request.Context(), inst.ContainerName)

	// 清空数据目录
	if inst.DataPath != "" {
		_ = os.RemoveAll(inst.DataPath)
		_ = os.MkdirAll(inst.DataPath, 0755)
	}

	// 解压
	if err := extractTarGz(file, inst.DataPath); err != nil {
		ErrResp(c, http.StatusInternalServerError, "FS_ERROR", "解压失败: "+err.Error())
		return
	}

	// 重启容器
	if err := docker.Start(c.Request.Context(), inst.ContainerName); err != nil {
		ErrResp(c, http.StatusInternalServerError, "DOCKER_ERROR", "重启失败: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, gin.H{"result": "ok"})
}

// RestoreCheckout POST /api/instances/restore-checkout
func (h *Handler) RestoreCheckout(c *gin.Context) {
	file, _, err := c.Request.FormFile("file")
	if err != nil {
		ErrResp(c, http.StatusBadRequest, "VALIDATION_ERROR", "请上传 .tar.gz 备份文件")
		return
	}
	defer file.Close()

	req := service.CheckoutReq{
		Platform:     c.PostForm("platform"),
		PlatformUser: c.PostForm("platform_user"),
		Slots:        15,
		Duration:     c.PostForm("duration"),
	}
	if req.Platform == "" || req.PlatformUser == "" {
		ErrResp(c, http.StatusBadRequest, "VALIDATION_ERROR", "platform 和 platform_user 不能为空")
		return
	}
	if s := c.PostForm("slots"); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			req.Slots = n
		}
	}
	if orderNo := c.PostForm("order_no"); orderNo != "" {
		req.OrderNo = &orderNo
	}
	if note := c.PostForm("note"); note != "" {
		req.Note = &note
	}

	// 执行 checkout（创建容器但不启动）
	resp, err := service.Checkout(c.Request.Context(), h.DB, h.Cfg, req)
	if err != nil {
		status, code := classifyError(err.Error())
		ErrResp(c, status, code, err.Error())
		return
	}

	// 停止容器 → 覆盖数据 → 重启
	_ = docker.Stop(c.Request.Context(), resp.Instance.ContainerName)
	if resp.Instance.DataPath != "" {
		_ = os.RemoveAll(resp.Instance.DataPath)
		_ = os.MkdirAll(resp.Instance.DataPath, 0755)
	}
	if err := extractTarGz(file, resp.Instance.DataPath); err != nil {
		ErrResp(c, http.StatusInternalServerError, "FS_ERROR", "解压失败: "+err.Error())
		return
	}
	if err := docker.Start(c.Request.Context(), resp.Instance.ContainerName); err != nil {
		ErrResp(c, http.StatusInternalServerError, "DOCKER_ERROR", "重启失败: "+err.Error())
		return
	}

	c.JSON(http.StatusOK, resp)
}

// extractTarGz 解压 tar.gz 到目标目录
func extractTarGz(r io.Reader, dst string) error {
	gr, err := gzip.NewReader(r)
	if err != nil {
		return err
	}
	defer gr.Close()
	tr := tar.NewReader(gr)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return err
		}

		// 安全检查：防止路径穿越
		target := filepath.Join(dst, filepath.FromSlash(header.Name))
		if !strings.HasPrefix(target, filepath.Clean(dst)+string(os.PathSeparator)) {
			continue
		}

		switch header.Typeflag {
		case tar.TypeDir:
			_ = os.MkdirAll(target, 0755)
		case tar.TypeReg:
			_ = os.MkdirAll(filepath.Dir(target), 0755)
			f, err := os.Create(target)
			if err != nil {
				return err
			}
			if _, err := io.Copy(f, tr); err != nil {
				f.Close()
				return err
			}
			f.Close()
		}
	}
	return nil
}
