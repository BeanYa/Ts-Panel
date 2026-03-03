package db

import "time"

// Customer 客户信息
type Customer struct {
	ID           string    `json:"id"`
	Platform     string    `json:"platform"`      // xianyu|taobao|other
	PlatformUser string    `json:"platform_user"` // 平台用户名/ID
	OrderNo      *string   `json:"order_no"`      // 可为空
	Note         *string   `json:"note"`
	CreatedAt    time.Time `json:"created_at"`
}

// Instance TeamSpeak 容器实例
type Instance struct {
	ID               string     `json:"id"`
	CustomerID       *string    `json:"customer_id"`
	ContainerName    string     `json:"container_name"`
	HostUDPPort      int        `json:"host_udp_port"`
	HostQueryPort    int        `json:"host_query_port"`
	Slots            int        `json:"slots"`
	SlotsApplied     bool       `json:"slots_applied"`
	Status           string     `json:"status"` // creating|running|stopped|recycled|failed
	CreatedAt        time.Time  `json:"created_at"`
	UpdatedAt        time.Time  `json:"updated_at"`
	ExpiresAt        *time.Time `json:"expires_at"`
	LastDeliveryText string     `json:"last_delivery_text"`
	DataPath         string     `json:"data_path"`
	ErrorMessage     *string    `json:"error_message"`
	LastAction       string     `json:"last_action"`
	// 密鑰（来自 secrets 表 JOIN）
	LoginName     *string `json:"login_name,omitempty"`
	AdminPassword *string `json:"admin_password,omitempty"`
	APIKey        *string `json:"api_key,omitempty"`
	PrivilegeKey  *string `json:"privilege_key,omitempty"`
	QueryPassword *string `json:"query_password,omitempty"`
}

// Secret 实例密钥信息
type Secret struct {
	InstanceID          string     `json:"instance_id"`
	ServerQueryPassword *string    `json:"serverquery_password"`
	AdminPrivilegeKey   *string    `json:"admin_privilege_key"`
	CapturedAt          *time.Time `json:"captured_at"`
}

// AuditLog 操作审计日志
type AuditLog struct {
	ID         string    `json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	Action     string    `json:"action"` // checkout|start|stop|restart|recycle|delete|capture_secrets|apply_slots
	InstanceID *string   `json:"instance_id"`
	CustomerID *string   `json:"customer_id"`
	Result     string    `json:"result"` // ok|err
	Detail     string    `json:"detail"`
}
