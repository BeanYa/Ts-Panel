package service_test

import (
	"testing"
	"ts-panel/src/service"
)

func TestParseSecrets_QueryPassword(t *testing.T) {
	logs := `
2024-01-01 12:00:00.000000 |INFO    |VirtualServer | 1|
2024-01-01 12:00:00.000000 |INFO    |VirtualServer | 1| ServerQuery password: rAnD0mP4ss
2024-01-01 12:00:00.000000 |INFO    |VirtualServer | 1| token=ABCDEFGHIJKLMNOPQRSTUVWXYZ1234567890
`
	result := service.ParseSecretsForTest(logs)

	if result.QueryPassword == nil {
		t.Error("期望找到 QueryPassword，实际为 nil")
	} else if *result.QueryPassword != "rAnD0mP4ss" {
		t.Errorf("QueryPassword 期望 rAnD0mP4ss, 实际 %s", *result.QueryPassword)
	}

	if result.PrivilegeKey == nil {
		t.Error("期望找到 PrivilegeKey，实际为 nil")
	}
}

func TestParseSecrets_Empty(t *testing.T) {
	result := service.ParseSecretsForTest("这是普通日志，没有密钥信息")
	if result.QueryPassword != nil {
		t.Error("空日志中不应找到 QueryPassword")
	}
	if result.PrivilegeKey != nil {
		t.Error("空日志中不应找到 PrivilegeKey")
	}
}

func TestBuildDeliveryText(t *testing.T) {
	text := service.BuildDeliveryTextForTest("1.2.3.4", 20100)
	if text == "" {
		t.Error("delivery_text 不应为空")
	}
	// 检查包含关键信息
	checks := []string{"1.2.3.4", "20100", "ts3server://"}
	for _, c := range checks {
		found := false
		for i := 0; i <= len(text)-len(c); i++ {
			if text[i:i+len(c)] == c {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("delivery_text 中缺少: %s", c)
		}
	}
}
