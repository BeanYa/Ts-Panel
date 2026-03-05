package db

import (
	"database/sql"
	"fmt"
	"strings"

	"ts-panel/src/config"
)

// RunMigrations 执行建表迁移（支持 MySQL 和 SQLite）
func RunMigrations(db *sql.DB, cfg *config.Config) error {
	dialect := GetDialect(cfg)

	migrations := []struct {
		name string
		sql  string
	}{
		{
			name: "create_customers",
			sql:  getCreateCustomersSQL(dialect),
		},
		{
			name: "create_customers_unique_idx",
			sql:  getCreateCustomersIdxSQL(dialect),
		},
		{
			name: "create_instances",
			sql:  getCreateInstancesSQL(dialect),
		},
		{
			name: "create_secrets",
			sql:  getCreateSecretsSQL(dialect),
		},
		{
			name: "create_audit_logs",
			sql:  getCreateAuditLogsSQL(dialect),
		},
	}

	for _, m := range migrations {
		_, err := db.Exec(m.sql)
		if err != nil {
			// 检查是否是已存在的错误（忽略）
			if isAlreadyExistsError(err, dialect) {
				continue
			}
			return fmt.Errorf("迁移 %s 失败: %w", m.name, err)
		}
	}

	// 执行列添加迁移（针对已存在的数据库）
	if err := runAlterMigrations(db, dialect); err != nil {
		return err
	}

	return nil
}

// runAlterMigrations 执行 ALTER TABLE 迁移
func runAlterMigrations(db *sql.DB, dialect string) error {
	alterMigrations := []struct {
		name     string
		sql      string
		sqliteFn func(*sql.DB) error
	}{
		{
			name: "alter_secrets_login_name",
			sql:  getAlterSecretsAddColumnSQL(dialect, "login_name", "TEXT"),
		},
		{
			name: "alter_secrets_admin_password",
			sql:  getAlterSecretsAddColumnSQL(dialect, "admin_password", "TEXT"),
		},
		{
			name: "alter_secrets_api_key",
			sql:  getAlterSecretsAddColumnSQL(dialect, "api_key", "TEXT"),
		},
	}

	for _, m := range alterMigrations {
		_, err := db.Exec(m.sql)
		if err != nil {
			// 列已存在时忽略错误
			if isDuplicateColumnError(err, dialect) {
				continue
			}
			return fmt.Errorf("迁移 %s 失败: %w", m.name, err)
		}
	}

	return nil
}

// ========== 表创建 SQL ==========

func getCreateCustomersSQL(dialect string) string {
	if dialect == "mysql" {
		return `CREATE TABLE IF NOT EXISTS customers (
			id            VARCHAR(255) PRIMARY KEY,
			platform      VARCHAR(255) NOT NULL,
			platform_user VARCHAR(255) NOT NULL,
			order_no      VARCHAR(255),
			note          TEXT,
			created_at    DATETIME NOT NULL,
			UNIQUE KEY idx_customers_platform_order (platform, order_no)
		)`
	}
	// SQLite
	return `CREATE TABLE IF NOT EXISTS customers (
		id            TEXT PRIMARY KEY,
		platform      TEXT NOT NULL,
		platform_user TEXT NOT NULL,
		order_no      TEXT,
		note          TEXT,
		created_at    TEXT NOT NULL
	)`
}

func getCreateCustomersIdxSQL(dialect string) string {
	if dialect == "mysql" {
		// MySQL 在 CREATE TABLE 中已包含 UNIQUE KEY
		return "SELECT 1"
	}
	// SQLite
	return `CREATE UNIQUE INDEX IF NOT EXISTS idx_customers_platform_order
		ON customers(platform, order_no)
		WHERE order_no IS NOT NULL`
}

func getCreateInstancesSQL(dialect string) string {
	if dialect == "mysql" {
		return `CREATE TABLE IF NOT EXISTS instances (
			id                  VARCHAR(255) PRIMARY KEY,
			customer_id         VARCHAR(255),
			container_name      VARCHAR(255) NOT NULL,
			host_udp_port       INT NOT NULL UNIQUE,
			host_query_port     INT NOT NULL UNIQUE,
			slots               INT NOT NULL DEFAULT 15,
			slots_applied       TINYINT NOT NULL DEFAULT 0,
			status              VARCHAR(50) NOT NULL DEFAULT 'creating',
			created_at          DATETIME NOT NULL,
			updated_at          DATETIME NOT NULL,
			expires_at          DATETIME,
			last_delivery_text  TEXT NOT NULL DEFAULT '',
			data_path           VARCHAR(500) NOT NULL DEFAULT '',
			error_message       TEXT,
			last_action         VARCHAR(255) NOT NULL DEFAULT ''
		)`
	}
	// SQLite
	return `CREATE TABLE IF NOT EXISTS instances (
		id                  TEXT PRIMARY KEY,
		customer_id         TEXT,
		container_name      TEXT NOT NULL,
		host_udp_port       INTEGER NOT NULL UNIQUE,
		host_query_port     INTEGER NOT NULL UNIQUE,
		slots               INTEGER NOT NULL DEFAULT 15,
		slots_applied       INTEGER NOT NULL DEFAULT 0,
		status              TEXT NOT NULL DEFAULT 'creating',
		created_at          TEXT NOT NULL,
		updated_at          TEXT NOT NULL,
		expires_at          TEXT,
		last_delivery_text  TEXT NOT NULL DEFAULT '',
		data_path           TEXT NOT NULL DEFAULT '',
		error_message       TEXT,
		last_action         TEXT NOT NULL DEFAULT ''
	)`
}

func getCreateSecretsSQL(dialect string) string {
	if dialect == "mysql" {
		return `CREATE TABLE IF NOT EXISTS secrets (
			instance_id             VARCHAR(255) PRIMARY KEY,
			login_name              VARCHAR(255),
			admin_password          VARCHAR(255),
			api_key                 VARCHAR(255),
			serverquery_password    VARCHAR(255),
			admin_privilege_key     VARCHAR(255),
			captured_at             DATETIME,
			FOREIGN KEY(instance_id) REFERENCES instances(id) ON DELETE CASCADE
		)`
	}
	// SQLite
	return `CREATE TABLE IF NOT EXISTS secrets (
		instance_id             TEXT PRIMARY KEY,
		login_name              TEXT,
		admin_password          TEXT,
		api_key                 TEXT,
		serverquery_password    TEXT,
		admin_privilege_key     TEXT,
		captured_at             TEXT,
		FOREIGN KEY(instance_id) REFERENCES instances(id)
	)`
}

func getCreateAuditLogsSQL(dialect string) string {
	if dialect == "mysql" {
		return `CREATE TABLE IF NOT EXISTS audit_logs (
			id          VARCHAR(255) PRIMARY KEY,
			created_at  DATETIME NOT NULL,
			action      VARCHAR(100) NOT NULL,
			instance_id VARCHAR(255),
			customer_id VARCHAR(255),
			result      VARCHAR(50) NOT NULL,
			detail      TEXT NOT NULL DEFAULT ''
		)`
	}
	// SQLite
	return `CREATE TABLE IF NOT EXISTS audit_logs (
		id          TEXT PRIMARY KEY,
		created_at  TEXT NOT NULL,
		action      TEXT NOT NULL,
		instance_id TEXT,
		customer_id TEXT,
		result      TEXT NOT NULL,
		detail      TEXT NOT NULL DEFAULT ''
	)`
}

func getAlterSecretsAddColumnSQL(dialect, column, dataType string) string {
	if dialect == "mysql" {
		return fmt.Sprintf("ALTER TABLE secrets ADD COLUMN %s %s", column, dataType)
	}
	// SQLite
	return fmt.Sprintf("ALTER TABLE secrets ADD COLUMN %s %s", column, dataType)
}

// ========== 错误检测 ==========

func isAlreadyExistsError(err error, dialect string) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	if dialect == "mysql" {
		return strings.Contains(errStr, "already exists") || strings.Contains(errStr, "duplicate key")
	}
	// SQLite
	return strings.Contains(errStr, "already exists")
}

func isDuplicateColumnError(err error, dialect string) bool {
	if err == nil {
		return false
	}
	errStr := strings.ToLower(err.Error())
	if dialect == "mysql" {
		return strings.Contains(errStr, "duplicate column") || strings.Contains(errStr, "already exists")
	}
	// SQLite
	return strings.Contains(errStr, "duplicate column")
}
