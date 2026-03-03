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
		SELECT i.id, i.customer_id, i.container_name, i.host_udp_port, i.host_query_port,
		       i.slots, i.slots_applied, i.status, i.created_at, i.updated_at,
		       i.expires_at, i.last_delivery_text, i.data_path, i.error_message, i.last_action,
		       s.login_name, s.admin_password, s.api_key,
		       s.serverquery_password, s.admin_privilege_key
		FROM instances i
		LEFT JOIN secrets s ON s.instance_id = i.id
		WHERE i.id = ?`, instanceID)
	return scanInstance(row)
}

// GetAllInstances 获取所有实例列表
func GetAllInstances(sqlDB *sql.DB) ([]*db.Instance, error) {
	rows, err := sqlDB.Query(`
		SELECT i.id, i.customer_id, i.container_name, i.host_udp_port, i.host_query_port,
		       i.slots, i.slots_applied, i.status, i.created_at, i.updated_at,
		       i.expires_at, i.last_delivery_text, i.data_path, i.error_message, i.last_action,
		       s.login_name, s.admin_password, s.api_key,
		       s.serverquery_password, s.admin_privilege_key
		FROM instances i
		LEFT JOIN secrets s ON s.instance_id = i.id
		ORDER BY i.created_at DESC`)
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
		var createdAtStr, updatedAtStr string
		var loginName, adminPass, apiKey, queryPass, privKey sql.NullString

		if err := rows.Scan(
			&inst.ID, &customerID, &inst.ContainerName,
			&inst.HostUDPPort, &inst.HostQueryPort,
			&inst.Slots, &slotsApplied, &inst.Status,
			&createdAtStr, &updatedAtStr,
			new(sql.NullString), // expires_at
			&inst.LastDeliveryText, &inst.DataPath,
			&errorMessage, &inst.LastAction,
			&loginName, &adminPass, &apiKey,
			&queryPass, &privKey,
		); err != nil {
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
		instances = append(instances, &inst)
	}
	return instances, rows.Err()
}

func updateInstanceStatus(sqlDB *sql.DB, instanceID, status, action string) {
	now := time.Now().UTC().Format(time.RFC3339)
	_, _ = sqlDB.Exec(`UPDATE instances SET status=?, last_action=?, updated_at=? WHERE id=?`,
		status, action, now, instanceID)
}
