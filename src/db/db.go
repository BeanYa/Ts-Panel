package db

import (
	"database/sql"
	"fmt"
	"os"
	"path/filepath"

	"ts-panel/src/config"

	_ "github.com/go-sql-driver/mysql"
	_ "modernc.org/sqlite"
)

// Open 根据配置打开数据库连接（支持 MySQL 和 SQLite）
func Open(cfg *config.Config) (*sql.DB, error) {
	switch cfg.DBType {
	case "mysql":
		return openMySQL(cfg)
	case "sqlite":
		return openSQLite(cfg.DBPath)
	default:
		return openSQLite(cfg.DBPath)
	}
}

// openMySQL 打开 MySQL 数据库连接
func openMySQL(cfg *config.Config) (*sql.DB, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=true&loc=Local",
		cfg.DBUser,
		cfg.DBPassword,
		cfg.DBHost,
		cfg.DBPort,
		cfg.DBName,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("打开 MySQL 数据库失败: %w", err)
	}

	// 测试连接
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("连接 MySQL 失败: %w", err)
	}

	// 设置连接池
	db.SetMaxOpenConns(25)
	db.SetMaxIdleConns(5)

	return db, nil
}

// openSQLite 打开 SQLite 数据库连接
func openSQLite(dbPath string) (*sql.DB, error) {
	// 确保目录存在
	dir := filepath.Dir(dbPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, fmt.Errorf("创建数据库目录失败: %w", err)
	}

	db, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, fmt.Errorf("打开 SQLite 数据库失败: %w", err)
	}

	// WAL 模式 + 性能优化
	pragmas := []string{
		"PRAGMA journal_mode=WAL",
		"PRAGMA synchronous=NORMAL",
		"PRAGMA foreign_keys=ON",
		"PRAGMA busy_timeout=5000",
	}
	for _, p := range pragmas {
		if _, err := db.Exec(p); err != nil {
			_ = db.Close()
			return nil, fmt.Errorf("设置 PRAGMA 失败 (%s): %w", p, err)
		}
	}

	return db, nil
}

// GetDialect 获取当前数据库方言（用于迁移兼容）
func GetDialect(cfg *config.Config) string {
	if cfg.DBType == "mysql" {
		return "mysql"
	}
	return "sqlite"
}
