package service

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"time"
	"ts-panel/src/db"
	"ts-panel/src/docker"
)

// Start 启动实例容器
func Start(ctx context.Context, sqlDB *sql.DB, instanceID string) error {
	inst, err := GetInstanceByID(sqlDB, instanceID)
	if err != nil || inst == nil {
		return fmt.Errorf("INSTANCE_NOT_FOUND")
	}

	if err := docker.Start(ctx, inst.ContainerName); err != nil {
		_ = writeAuditLog(sqlDB, "start", &instanceID, inst.CustomerID, "err", err.Error())
		return fmt.Errorf("DOCKER_ERROR: %w", err)
	}

	updateInstanceStatus(sqlDB, instanceID, "running", "start")
	_ = writeAuditLog(sqlDB, "start", &instanceID, inst.CustomerID, "ok", "")
	return nil
}

// Stop 停止实例容器
func Stop(ctx context.Context, sqlDB *sql.DB, instanceID string) error {
	inst, err := GetInstanceByID(sqlDB, instanceID)
	if err != nil || inst == nil {
		return fmt.Errorf("INSTANCE_NOT_FOUND")
	}

	if err := docker.Stop(ctx, inst.ContainerName); err != nil {
		_ = writeAuditLog(sqlDB, "stop", &instanceID, inst.CustomerID, "err", err.Error())
		return fmt.Errorf("DOCKER_ERROR: %w", err)
	}

	updateInstanceStatus(sqlDB, instanceID, "stopped", "stop")
	_ = writeAuditLog(sqlDB, "stop", &instanceID, inst.CustomerID, "ok", "")
	return nil
}

// Restart 重启实例容器
func Restart(ctx context.Context, sqlDB *sql.DB, instanceID string) error {
	inst, err := GetInstanceByID(sqlDB, instanceID)
	if err != nil || inst == nil {
		return fmt.Errorf("INSTANCE_NOT_FOUND")
	}

	if err := docker.Restart(ctx, inst.ContainerName); err != nil {
		_ = writeAuditLog(sqlDB, "restart", &instanceID, inst.CustomerID, "err", err.Error())
		return fmt.Errorf("DOCKER_ERROR: %w", err)
	}

	updateInstanceStatus(sqlDB, instanceID, "running", "restart")
	_ = writeAuditLog(sqlDB, "restart", &instanceID, inst.CustomerID, "ok", "")
	return nil
}

// Recycle 回收实例（停止容器，解绑客户）
func Recycle(ctx context.Context, sqlDB *sql.DB, instanceID string, wipeData bool) error {
	inst, err := GetInstanceByID(sqlDB, instanceID)
	if err != nil || inst == nil {
		return fmt.Errorf("INSTANCE_NOT_FOUND")
	}

	if err := docker.Stop(ctx, inst.ContainerName); err != nil {
		_ = writeAuditLog(sqlDB, "recycle", &instanceID, inst.CustomerID, "err", err.Error())
		return fmt.Errorf("DOCKER_ERROR: %w", err)
	}

	if wipeData && inst.DataPath != "" {
		_ = os.RemoveAll(inst.DataPath)
	}

	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = sqlDB.Exec(`
		UPDATE instances SET status='recycled', customer_id=NULL, last_action='recycle', updated_at=?
		WHERE id=?`, now, instanceID)

	_ = writeAuditLog(sqlDB, "recycle", &instanceID, inst.CustomerID, "ok", fmt.Sprintf("wipe_data=%v", wipeData))
	return nil
}

// Delete 删除实例（需明确 confirm=true）
func Delete(ctx context.Context, sqlDB *sql.DB, instanceID string) error {
	inst, err := GetInstanceByID(sqlDB, instanceID)
	if err != nil || inst == nil {
		return fmt.Errorf("INSTANCE_NOT_FOUND")
	}

	// 强制删除容器
	_ = docker.Remove(ctx, inst.ContainerName)

	// 删除数据目录
	if inst.DataPath != "" {
		_ = os.RemoveAll(inst.DataPath)
	}

	// 删除 DB 记录（级联释放端口）
	_, _ = sqlDB.Exec(`DELETE FROM secrets WHERE instance_id = ?`, instanceID)
	_, _ = sqlDB.Exec(`DELETE FROM instances WHERE id = ?`, instanceID)

	_ = writeAuditLog(sqlDB, "delete", &instanceID, inst.CustomerID, "ok", "实例已彻底删除")
	return nil
}

// GetInstanceByID 按 ID 查询实例
func GetInstanceByID(sqlDB *sql.DB, instanceID string) (*db.Instance, error) {
	row := sqlDB.QueryRow(`
		SELECT id, customer_id, container_name, host_udp_port, host_query_port,
		       slots, slots_applied, status, created_at, updated_at,
		       expires_at, last_delivery_text, data_path, error_message, last_action
		FROM instances WHERE id = ?`, instanceID)
	return scanInstance(row)
}

// GetAllInstances 获取所有实例列表
func GetAllInstances(sqlDB *sql.DB) ([]*db.Instance, error) {
	rows, err := sqlDB.Query(`
		SELECT id, customer_id, container_name, host_udp_port, host_query_port,
		       slots, slots_applied, status, created_at, updated_at,
		       expires_at, last_delivery_text, data_path, error_message, last_action
		FROM instances ORDER BY created_at DESC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var instances []*db.Instance
	for rows.Next() {
		var inst db.Instance
		var customerID sql.NullString
		var errorMessage sql.NullString
		var slotsApplied int

		if err := rows.Scan(
			&inst.ID, &customerID, &inst.ContainerName,
			&inst.HostUDPPort, &inst.HostQueryPort,
			&inst.Slots, &slotsApplied, &inst.Status,
			&inst.CreatedAt, &inst.UpdatedAt,
			new(sql.NullString), // expires_at
			&inst.LastDeliveryText, &inst.DataPath,
			&errorMessage, &inst.LastAction,
		); err != nil {
			return nil, err
		}
		if customerID.Valid {
			inst.CustomerID = &customerID.String
		}
		if errorMessage.Valid {
			inst.ErrorMessage = &errorMessage.String
		}
		inst.SlotsApplied = slotsApplied == 1
		instances = append(instances, &inst)
	}
	return instances, rows.Err()
}

func updateInstanceStatus(sqlDB *sql.DB, instanceID, status, action string) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = sqlDB.Exec(`UPDATE instances SET status=?, last_action=?, updated_at=? WHERE id=?`,
		status, action, now, instanceID)
}
