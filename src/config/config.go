package config

import (
	"fmt"
	"os"
	"strconv"
)

// Config 存储所有运行时配置
type Config struct {
	// 公网 IP
	PublicIP string

	// 管理 Token
	AdminToken string

	// HTTP 监听端口
	HTTPPort string

	// 语音 UDP 端口池（公网）
	PortMin int
	PortMax int

	// ServerQuery TCP 端口池（仅本机）
	QueryPortMin int
	QueryPortMax int

	// 容器资源限制默认值
	DefaultCPU    string
	DefaultMemory string
	DefaultPids   int

	// 重试次数
	CreateRetry  int
	SecretsRetry int
	LogTail      int

	// 数据目录
	DataRoot string
	DBPath   string
}

// Load 从环境变量中加载配置
func Load() (*Config, error) {
	cfg := &Config{
		PublicIP:      requireEnv("PUBLIC_IP"),
		AdminToken:    requireEnv("ADMIN_TOKEN"),
		HTTPPort:      getEnv("HTTP_PORT", "8080"),
		PortMin:       getIntEnv("PORT_MIN", 20000),
		PortMax:       getIntEnv("PORT_MAX", 20999),
		QueryPortMin:  getIntEnv("QUERY_PORT_MIN", 21000),
		QueryPortMax:  getIntEnv("QUERY_PORT_MAX", 21999),
		DefaultCPU:    getEnv("DEFAULT_CPU", "0.5"),
		DefaultMemory: getEnv("DEFAULT_MEMORY", "512m"),
		DefaultPids:   getIntEnv("DEFAULT_PIDS", 200),
		CreateRetry:   getIntEnv("CREATE_RETRY", 2),
		SecretsRetry:  getIntEnv("SECRETS_RETRY", 10),
		LogTail:       getIntEnv("LOG_TAIL", 300),
		DataRoot:      getEnv("DATA_ROOT", "/data"),
		DBPath:        getEnv("DB_PATH", "/data/db/app.db"),
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.PublicIP == "" {
		return fmt.Errorf("PUBLIC_IP 不能为空")
	}
	if c.AdminToken == "" {
		return fmt.Errorf("ADMIN_TOKEN 不能为空")
	}
	if c.PortMin >= c.PortMax {
		return fmt.Errorf("PORT_MIN(%d) 必须小于 PORT_MAX(%d)", c.PortMin, c.PortMax)
	}
	if c.QueryPortMin >= c.QueryPortMax {
		return fmt.Errorf("QUERY_PORT_MIN(%d) 必须小于 QUERY_PORT_MAX(%d)", c.QueryPortMin, c.QueryPortMax)
	}
	return nil
}

func requireEnv(key string) string {
	return os.Getenv(key)
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func getIntEnv(key string, def int) int {
	if v := os.Getenv(key); v != "" {
		if n, err := strconv.Atoi(v); err == nil {
			return n
		}
	}
	return def
}
