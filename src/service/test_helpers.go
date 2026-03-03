// test_helpers.go 提供测试用的导出函数（仅用于测试）
package service

// ParseSecretsForTest 导出 parseSecrets 供测试使用
func ParseSecretsForTest(logs string) *CaptureResult {
	return parseSecrets(logs)
}

// BuildDeliveryTextForTest 导出 buildDeliveryText 供测试使用
func BuildDeliveryTextForTest(publicIP string, udpPort int) string {
	return buildDeliveryText(publicIP, udpPort)
}
