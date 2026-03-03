package port_test

import (
	"database/sql"
	"testing"
	"ts-panel/src/port"

	_ "modernc.org/sqlite"
)

func setupTestDB(t *testing.T) *sql.DB {
	t.Helper()
	db, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("打开测试 DB 失败: %v", err)
	}
	_, err = db.Exec(`CREATE TABLE instances (
		id TEXT PRIMARY KEY,
		host_udp_port INTEGER NOT NULL UNIQUE,
		host_query_port INTEGER NOT NULL UNIQUE
	)`)
	if err != nil {
		t.Fatalf("创建测试表失败: %v", err)
	}
	return db
}

func TestAllocate_Basic(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	tx, _ := db.Begin()
	result, err := port.Allocate(tx, 20000, 20999, 21000, 21999)
	if err != nil {
		t.Fatalf("分配端口失败: %v", err)
	}
	if result.UDPPort != 20000 {
		t.Errorf("期望 UDP 端口 20000, 实际 %d", result.UDPPort)
	}
	if result.QueryPort != 21000 {
		t.Errorf("期望 Query 端口 21000, 实际 %d", result.QueryPort)
	}
	_ = tx.Rollback()
}

func TestAllocate_SkipsUsedPorts(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// 插入已占用的端口
	_, _ = db.Exec(`INSERT INTO instances (id, host_udp_port, host_query_port) VALUES ('inst1', 20000, 21000)`)
	_, _ = db.Exec(`INSERT INTO instances (id, host_udp_port, host_query_port) VALUES ('inst2', 20001, 21001)`)

	tx, _ := db.Begin()
	result, err := port.Allocate(tx, 20000, 20999, 21000, 21999)
	if err != nil {
		t.Fatalf("分配端口失败: %v", err)
	}
	if result.UDPPort != 20002 {
		t.Errorf("期望跳过已用端口，UDP 应分配 20002, 实际 %d", result.UDPPort)
	}
	if result.QueryPort != 21002 {
		t.Errorf("期望跳过已用端口，Query 应分配 21002, 实际 %d", result.QueryPort)
	}
	_ = tx.Rollback()
}

func TestAllocate_PoolExhausted(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// 只有 1 个 UDP 端口，且已被占用
	_, _ = db.Exec(`INSERT INTO instances (id, host_udp_port, host_query_port) VALUES ('inst1', 20000, 21000)`)

	tx, _ := db.Begin()
	_, err := port.Allocate(tx, 20000, 20000, 21000, 21999)
	if err == nil {
		t.Error("期望端口耗尽错误，但未返回错误")
	}
	_ = tx.Rollback()
}

func TestAllocate_NoConflictInParallel(t *testing.T) {
	db := setupTestDB(t)
	defer db.Close()

	// 两个并发事务分别分配不同端口
	tx1, _ := db.Begin()
	r1, err := port.Allocate(tx1, 20000, 20002, 21000, 21002)
	if err != nil {
		t.Fatalf("tx1 分配失败: %v", err)
	}

	// tx1 还未提交，模拟插入端口到已占用状态
	_, _ = db.Exec(`INSERT INTO instances (id, host_udp_port, host_query_port) VALUES ('inst1', ?, ?)`, r1.UDPPort, r1.QueryPort)
	_ = tx1.Rollback()

	tx2, _ := db.Begin()
	r2, err := port.Allocate(tx2, 20000, 20002, 21000, 21002)
	if err != nil {
		t.Fatalf("tx2 分配失败: %v", err)
	}
	if r2.UDPPort == r1.UDPPort {
		t.Errorf("并发分配了相同的 UDP 端口: %d", r2.UDPPort)
	}
	_ = tx2.Rollback()
}
