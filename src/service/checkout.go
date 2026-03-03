package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"path/filepath"
	"time"
	"ts-panel/src/config"
	"ts-panel/src/db"
	"ts-panel/src/docker"
	"ts-panel/src/port"

	"github.com/google/uuid"
)

// CheckoutReq checkout 请求
type CheckoutReq struct {
	Platform     string     `json:"platform" binding:"required"`
	PlatformUser string     `json:"platform_user" binding:"required"`
	OrderNo      *string    `json:"order_no"`
	Note         *string    `json:"note"`
	Slots        int        `json:"slots"`
	ExpiresAt    *time.Time `json:"expires_at"`
	ReuseRecycled bool      `json:"reuse_recycled"`
	// 覆盖默认资源限制
	CPU    string `json:"cpu"`
	Memory string `json:"memory"`
	Pids   int    `json:"pids"`
}

// CheckoutResp checkout 返回
type CheckoutResp struct {
	Instance     *db.Instance   `json:"instance"`
	DeliveryText string         `json:"delivery_text"`
	Secrets      *CaptureResult `json:"secrets,omitempty"`
	Warnings     []string       `json:"warnings,omitempty"`
	Reused       bool           `json:"reused"`
}

// Checkout 核心发货流程（幂等、事务化）
func Checkout(ctx context.Context, sqlDB *sql.DB, cfg *config.Config, req CheckoutReq) (*CheckoutResp, error) {
	// 默认值
	if req.Slots <= 0 {
		req.Slots = 15
	}
	if req.CPU == "" {
		req.CPU = cfg.DefaultCPU
	}
	if req.Memory == "" {
		req.Memory = cfg.DefaultMemory
	}
	if req.Pids <= 0 {
		req.Pids = cfg.DefaultPids
	}

	// 1. 幂等检查：相同 platform+order_no 已存在实例
	if req.OrderNo != nil && *req.OrderNo != "" {
		existing, err := findExistingInstance(sqlDB, req.Platform, *req.OrderNo)
		if err != nil {
			return nil, fmt.Errorf("DB_ERROR: %w", err)
		}
		if existing != nil {
			deliveryText := buildDeliveryText(cfg.PublicIP, existing.HostUDPPort)
			return &CheckoutResp{
				Instance:     existing,
				DeliveryText: deliveryText,
				Reused:       true,
			}, nil
		}
	}

	// 2. 复用 recycled 实例（如果请求了）
	if req.ReuseRecycled {
		instance, err := reuseRecycled(ctx, sqlDB, cfg, req)
		if err == nil && instance != nil {
			deliveryText := buildDeliveryText(cfg.PublicIP, instance.HostUDPPort)
			return &CheckoutResp{
				Instance:     instance,
				DeliveryText: deliveryText,
				Reused:       true,
			}, nil
		}
	}

	// 3. 新建实例
	return createNew(ctx, sqlDB, cfg, req)
}

// findExistingInstance 查找同 platform+order_no 的活跃实例
func findExistingInstance(sqlDB *sql.DB, platform, orderNo string) (*db.Instance, error) {
	row := sqlDB.QueryRow(`
		SELECT i.id, i.customer_id, i.container_name, i.host_udp_port, i.host_query_port,
		       i.slots, i.slots_applied, i.status, i.created_at, i.updated_at,
		       i.expires_at, i.last_delivery_text, i.data_path, i.error_message, i.last_action
		FROM instances i
		JOIN customers c ON c.id = i.customer_id
		WHERE c.platform = ? AND c.order_no = ? AND i.status != 'recycled' AND i.status != 'failed'
		LIMIT 1`, platform, orderNo)

	return scanInstance(row)
}

// reuseRecycled 复用一个 recycled 状态的实例
func reuseRecycled(ctx context.Context, sqlDB *sql.DB, cfg *config.Config, req CheckoutReq) (*db.Instance, error) {
	row := sqlDB.QueryRow(`
		SELECT id, customer_id, container_name, host_udp_port, host_query_port,
		       slots, slots_applied, status, created_at, updated_at,
		       expires_at, last_delivery_text, data_path, error_message, last_action
		FROM instances WHERE status = 'recycled' LIMIT 1`)

	instance, err := scanInstance(row)
	if err != nil || instance == nil {
		return nil, fmt.Errorf("没有可复用实例")
	}

	// 创建客户
	customerID, err := ensureCustomer(sqlDB, req)
	if err != nil {
		return nil, err
	}

	// 启动容器
	if err := docker.Start(ctx, instance.ContainerName); err != nil {
		return nil, err
	}

	deliveryText := buildDeliveryText(cfg.PublicIP, instance.HostUDPPort)
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = sqlDB.Exec(`
		UPDATE instances SET status='running', customer_id=?, last_delivery_text=?, last_action='reuse', updated_at=?
		WHERE id=?`, customerID, deliveryText, now, instance.ID)

	instance.Status = "running"
	instance.LastDeliveryText = deliveryText
	_ = writeAuditLog(sqlDB, "checkout", &instance.ID, &customerID, "ok", "复用实例")
	return instance, nil
}

// createNew 全新创建一个实例
func createNew(ctx context.Context, sqlDB *sql.DB, cfg *config.Config, req CheckoutReq) (*CheckoutResp, error) {
	instanceID := uuid.New().String()
	containerName := fmt.Sprintf("ts-%s", instanceID[:8])
	dataPath := filepath.Join(cfg.DataRoot, "instances", instanceID)

	// 确保数据目录存在
	if err := os.MkdirAll(dataPath, 0755); err != nil {
		return nil, fmt.Errorf("FS_ERROR: 创建数据目录失败: %w", err)
	}

	// 事务内分配端口 + 插入实例记录
	tx, err := sqlDB.BeginTx(ctx, &sql.TxOptions{Isolation: sql.LevelSerializable})
	if err != nil {
		return nil, fmt.Errorf("DB_ERROR: 开启事务失败: %w", err)
	}

	ports, err := port.Allocate(tx, cfg.PortMin, cfg.PortMax, cfg.QueryPortMin, cfg.QueryPortMax)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("NO_PORT_AVAILABLE: %w", err)
	}

	customerID, err := ensureCustomerTx(tx, req)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("DB_ERROR: %w", err)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, err = tx.Exec(`
		INSERT INTO instances
		(id, customer_id, container_name, host_udp_port, host_query_port, slots, status, created_at, updated_at, data_path, last_action)
		VALUES (?, ?, ?, ?, ?, ?, 'creating', ?, ?, ?, 'checkout')`,
		instanceID, customerID, containerName,
		ports.UDPPort, ports.QueryPort,
		req.Slots, now, now, dataPath)
	if err != nil {
		_ = tx.Rollback()
		return nil, fmt.Errorf("DB_ERROR: 插入实例失败: %w", err)
	}

	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("DB_ERROR: 提交事务失败: %w", err)
	}

	// 创建并启动容器
	dockerParams := docker.ContainerParams{
		Name:       containerName,
		InstanceID: instanceID,
		CustomerID: customerID,
		DataPath:   dataPath,
		UDPPort:    ports.UDPPort,
		QueryPort:  ports.QueryPort,
		CPU:        req.CPU,
		Memory:     req.Memory,
		Pids:       req.Pids,
		Slots:      req.Slots,
	}

	var warnings []string
	if err := docker.CreateAndStart(ctx, dockerParams, cfg.CreateRetry); err != nil {
		errMsg := err.Error()
		_, _ = sqlDB.Exec(`UPDATE instances SET status='failed', error_message=?, updated_at=? WHERE id=?`,
			errMsg, time.Now().UTC().Format(time.RFC3339), instanceID)
		_ = writeAuditLog(sqlDB, "checkout", &instanceID, &customerID, "err", errMsg)
		return nil, fmt.Errorf("DOCKER_ERROR: %w", err)
	}

	deliveryText := buildDeliveryText(cfg.PublicIP, ports.UDPPort)
	now = time.Now().UTC().Format(time.RFC3339)
	_, _ = sqlDB.Exec(`UPDATE instances SET status='running', last_delivery_text=?, updated_at=? WHERE id=?`,
		deliveryText, now, instanceID)

	// Best-effort: 抓取 secrets
	captureResult, captureErr := CaptureSecrets(ctx, containerName, cfg.LogTail, cfg.SecretsRetry)
	if captureErr != nil {
		warnings = append(warnings, fmt.Sprintf("secrets 未抓取: %s", captureErr.Error()))
	} else if captureResult != nil {
		_ = SaveSecrets(sqlDB, instanceID, captureResult)
		// Best-effort: 应用 slots
		slotsResult := ApplySlots(ctx, sqlDB, instanceID, containerName, ports.QueryPort, req.Slots, 10)
		if !slotsResult.Applied {
			warnings = append(warnings, fmt.Sprintf("slots 未应用: %s", slotsResult.Error))
		}
	}

	_ = writeAuditLog(sqlDB, "checkout", &instanceID, &customerID, "ok", fmt.Sprintf("实例创建成功，端口 %d", ports.UDPPort))

	instance := &db.Instance{
		ID:               instanceID,
		CustomerID:       &customerID,
		ContainerName:    containerName,
		HostUDPPort:      ports.UDPPort,
		HostQueryPort:    ports.QueryPort,
		Slots:            req.Slots,
		Status:           "running",
		CreatedAt:        time.Now().UTC(),
		UpdatedAt:        time.Now().UTC(),
		LastDeliveryText: deliveryText,
		DataPath:         dataPath,
		LastAction:       "checkout",
	}

	resp := &CheckoutResp{
		Instance:     instance,
		DeliveryText: deliveryText,
		Warnings:     warnings,
	}
	if captureResult != nil {
		resp.Secrets = captureResult
	}
	return resp, nil
}

// buildDeliveryText 生成发货文本
func buildDeliveryText(publicIP string, udpPort int) string {
	return fmt.Sprintf(`TeamSpeak 服务器已开通 ✅
服务器地址：%s:%d
一键连接：ts3server://%s?port=%d

如连接失败回复"重发"或"重启"我这边处理。`, publicIP, udpPort, publicIP, udpPort)
}

// ensureCustomer 创建或获取客户（非事务版）
func ensureCustomer(sqlDB *sql.DB, req CheckoutReq) (string, error) {
	customerID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := sqlDB.Exec(`
		INSERT INTO customers (id, platform, platform_user, order_no, note, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		customerID, req.Platform, req.PlatformUser, req.OrderNo, req.Note, now)
	if err != nil {
		return "", err
	}
	return customerID, nil
}

// ensureCustomerTx 在事务中创建客户
func ensureCustomerTx(tx *sql.Tx, req CheckoutReq) (string, error) {
	customerID := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := tx.Exec(`
		INSERT INTO customers (id, platform, platform_user, order_no, note, created_at)
		VALUES (?, ?, ?, ?, ?, ?)`,
		customerID, req.Platform, req.PlatformUser, req.OrderNo, req.Note, now)
	if err != nil {
		return "", err
	}
	return customerID, nil
}

// writeAuditLog 写审计日志
func writeAuditLog(sqlDB *sql.DB, action string, instanceID, customerID *string, result, detail string) error {
	id := uuid.New().String()
	now := time.Now().UTC().Format(time.RFC3339)
	_, err := sqlDB.Exec(`
		INSERT INTO audit_logs (id, created_at, action, instance_id, customer_id, result, detail)
		VALUES (?, ?, ?, ?, ?, ?, ?)`,
		id, now, action, instanceID, customerID, result, detail)
	return err
}

// scanInstance 从 Row 扫描 Instance
func scanInstance(row *sql.Row) (*db.Instance, error) {
	var inst db.Instance
	var customerID sql.NullString
	var errorMessage sql.NullString
	var slotsApplied int
	var createdAtStr, updatedAtStr string
	var loginName, adminPass, apiKey, queryPass, privKey sql.NullString

	err := row.Scan(
		&inst.ID, &customerID, &inst.ContainerName,
		&inst.HostUDPPort, &inst.HostQueryPort,
		&inst.Slots, &slotsApplied, &inst.Status,
		&createdAtStr, &updatedAtStr,
		new(sql.NullString), // expires_at 暂不使用
		&inst.LastDeliveryText, &inst.DataPath,
		&errorMessage, &inst.LastAction,
		&loginName, &adminPass, &apiKey,
		&queryPass, &privKey,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}

	inst.CreatedAt, _ = time.Parse(time.RFC3339, createdAtStr)
	inst.UpdatedAt, _ = time.Parse(time.RFC3339, updatedAtStr)
	if customerID.Valid {
		inst.CustomerID = &customerID.String
	}
	if errorMessage.Valid {
		inst.ErrorMessage = &errorMessage.String
	}
	if loginName.Valid && loginName.String != "" {
		inst.LoginName = &loginName.String
	}
	if adminPass.Valid && adminPass.String != "" {
		inst.AdminPassword = &adminPass.String
	}
	if apiKey.Valid && apiKey.String != "" {
		inst.APIKey = &apiKey.String
	}
	if queryPass.Valid && queryPass.String != "" {
		inst.QueryPassword = &queryPass.String
	}
	if privKey.Valid && privKey.String != "" {
		inst.PrivilegeKey = &privKey.String
	}
	inst.SlotsApplied = slotsApplied == 1
	return &inst, nil
}

