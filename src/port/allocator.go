package port

import (
	"database/sql"
	"errors"
	"fmt"
)

// ErrNoPortAvailable 表示端口池已耗尽
var ErrNoPortAvailable = errors.New("NO_PORT_AVAILABLE")

// AllocateResult 分配结果
type AllocateResult struct {
	UDPPort   int
	QueryPort int
}

// Allocate 在事务内分配一对空闲端口（UDP + Query）
// 必须在 BEGIN IMMEDIATE 事务内调用
func Allocate(tx *sql.Tx, udpMin, udpMax, queryMin, queryMax int) (*AllocateResult, error) {
	udpPort, err := findFreePort(tx, udpMin, udpMax, "host_udp_port")
	if err != nil {
		return nil, fmt.Errorf("UDP 端口: %w", err)
	}

	queryPort, err := findFreePort(tx, queryMin, queryMax, "host_query_port")
	if err != nil {
		return nil, fmt.Errorf("Query 端口: %w", err)
	}

	return &AllocateResult{
		UDPPort:   udpPort,
		QueryPort: queryPort,
	}, nil
}

// findFreePort 查找某个字段列中第一个未被占用的端口
func findFreePort(tx *sql.Tx, min, max int, col string) (int, error) {
	// 获取已用端口集合
	query := fmt.Sprintf(`SELECT %s FROM instances WHERE %s >= ? AND %s <= ?`, col, col, col)
	rows, err := tx.Query(query, min, max)
	if err != nil {
		return 0, fmt.Errorf("查询已用端口失败: %w", err)
	}
	defer rows.Close()

	used := make(map[int]bool)
	for rows.Next() {
		var p int
		if err := rows.Scan(&p); err != nil {
			return 0, err
		}
		used[p] = true
	}
	if err := rows.Err(); err != nil {
		return 0, err
	}

	// 从 min 到 max 找第一个空闲端口
	for p := min; p <= max; p++ {
		if !used[p] {
			return p, nil
		}
	}
	return 0, ErrNoPortAvailable
}
