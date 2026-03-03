package db

import (
	"database/sql"
	"fmt"
)

// RunMigrations 执行建表迁移（幂等）
func RunMigrations(db *sql.DB) error {
	migrations := []struct {
		name string
		sql  string
	}{
		{
			"create_customers",
			`CREATE TABLE IF NOT EXISTS customers (
				id            TEXT PRIMARY KEY,
				platform      TEXT NOT NULL,
				platform_user TEXT NOT NULL,
				order_no      TEXT,
				note          TEXT,
				created_at    TEXT NOT NULL
			)`,
		},
		{
			"create_customers_unique_idx",
			`CREATE UNIQUE INDEX IF NOT EXISTS idx_customers_platform_order
				ON customers(platform, order_no)
				WHERE order_no IS NOT NULL`,
		},
		{
			"create_instances",
			`CREATE TABLE IF NOT EXISTS instances (
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
			)`,
		},
		{
			"create_secrets",
			`CREATE TABLE IF NOT EXISTS secrets (
				instance_id             TEXT PRIMARY KEY,
				serverquery_password    TEXT,
				admin_privilege_key     TEXT,
				captured_at             TEXT,
				FOREIGN KEY(instance_id) REFERENCES instances(id)
			)`,
		},
		{
			"create_audit_logs",
			`CREATE TABLE IF NOT EXISTS audit_logs (
				id          TEXT PRIMARY KEY,
				created_at  TEXT NOT NULL,
				action      TEXT NOT NULL,
				instance_id TEXT,
				customer_id TEXT,
				result      TEXT NOT NULL,
				detail      TEXT NOT NULL DEFAULT ''
			)`,
		},
	}

	for _, m := range migrations {
		if _, err := db.Exec(m.sql); err != nil {
			return fmt.Errorf("迁移 %s 失败: %w", m.name, err)
		}
	}
	return nil
}
