package db_test

import (
	"database/sql"
	"testing"
	"ts-panel/src/db"

	_ "modernc.org/sqlite"
)

func openTestDB(t *testing.T) *sql.DB {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("打开测试 DB 失败: %v", err)
	}
	// WAL 用于内存 DB 可跳过
	return sqlDB
}

func TestRunMigrations_Idempotent(t *testing.T) {
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	// 必须可以多次执行
	for i := 0; i < 3; i++ {
		if err := db.RunMigrations(sqlDB); err != nil {
			t.Fatalf("第 %d 次迁移失败: %v", i+1, err)
		}
	}
}

func TestRunMigrations_TablesExist(t *testing.T) {
	sqlDB := openTestDB(t)
	defer sqlDB.Close()

	if err := db.RunMigrations(sqlDB); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	tables := []string{"customers", "instances", "secrets", "audit_logs"}
	for _, table := range tables {
		var name string
		err := sqlDB.QueryRow(
			`SELECT name FROM sqlite_master WHERE type='table' AND name=?`, table,
		).Scan(&name)
		if err != nil || name != table {
			t.Errorf("缺少表: %s", table)
		}
	}
}

func TestInstances_UniquePortConstraint(t *testing.T) {
	sqlDB := openTestDB(t)
	defer sqlDB.Close()
	_ = db.RunMigrations(sqlDB)

	now := "2024-01-01T00:00:00Z"
	_, err := sqlDB.Exec(`
		INSERT INTO instances (id, container_name, host_udp_port, host_query_port, created_at, updated_at)
		VALUES ('id1', 'ts-1', 20000, 21000, ?, ?)`, now, now)
	if err != nil {
		t.Fatalf("第一次插入失败: %v", err)
	}

	// 相同 UDP 端口 → 应违反 UNIQUE 约束
	_, err = sqlDB.Exec(`
		INSERT INTO instances (id, container_name, host_udp_port, host_query_port, created_at, updated_at)
		VALUES ('id2', 'ts-2', 20000, 21001, ?, ?)`, now, now)
	if err == nil {
		t.Error("期望 UNIQUE 约束错误，但插入成功")
	}
}
