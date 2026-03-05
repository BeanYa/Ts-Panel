package api_test

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"ts-panel/src/api"
	"ts-panel/src/config"
	"ts-panel/src/db"

	_ "modernc.org/sqlite"
)

func setupTestServer(t *testing.T) (*httptest.Server, *sql.DB) {
	t.Helper()
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("打开测试 DB 失败: %v", err)
	}

	cfg := &config.Config{
		PublicIP:      "1.2.3.4",
		AdminToken:    "test-token-123",
		HTTPPort:      "8080",
		PortMin:       20000,
		PortMax:       20999,
		QueryPortMin:  21000,
		QueryPortMax:  21999,
		DefaultCPU:    "0.5",
		DefaultMemory: "512m",
		DefaultPids:   200,
		CreateRetry:   0,
		SecretsRetry:  1,
		LogTail:       50,
		DataRoot:      t.TempDir(),
		DBPath:        ":memory:",
		DBType:        "sqlite",
	}

	if err := db.RunMigrations(sqlDB, cfg); err != nil {
		t.Fatalf("迁移失败: %v", err)
	}

	router := api.SetupRouter(sqlDB, cfg)
	srv := httptest.NewServer(router)
	t.Cleanup(func() {
		srv.Close()
		sqlDB.Close()
	})
	return srv, sqlDB
}

func TestHealthCheck(t *testing.T) {
	srv, _ := setupTestServer(t)

	resp, err := http.Get(srv.URL + "/healthz")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望 200, 实际 %d", resp.StatusCode)
	}
}

func TestAuth_MissingToken(t *testing.T) {
	srv, _ := setupTestServer(t)

	resp, err := http.Get(srv.URL + "/api/instances")
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("期望 401, 实际 %d", resp.StatusCode)
	}
}

func TestAuth_WrongToken(t *testing.T) {
	srv, _ := setupTestServer(t)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/instances", nil)
	req.Header.Set("X-Admin-Token", "wrong-token")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("期望 401, 实际 %d", resp.StatusCode)
	}
}

func TestAuth_ValidToken_ListInstances(t *testing.T) {
	srv, _ := setupTestServer(t)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/instances", nil)
	req.Header.Set("X-Admin-Token", "test-token-123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("期望 200, 实际 %d", resp.StatusCode)
	}

	var body map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if _, ok := body["instances"]; !ok {
		t.Error("响应中缺少 instances 字段")
	}
}

func TestGetInstance_NotFound(t *testing.T) {
	srv, _ := setupTestServer(t)

	req, _ := http.NewRequest(http.MethodGet, srv.URL+"/api/instances/nonexistent-id", nil)
	req.Header.Set("X-Admin-Token", "test-token-123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("期望 404, 实际 %d", resp.StatusCode)
	}
}

func TestDeleteInstance_MissingConfirm(t *testing.T) {
	srv, _ := setupTestServer(t)

	req, _ := http.NewRequest(http.MethodDelete, srv.URL+"/api/instances/some-id", nil)
	req.Header.Set("X-Admin-Token", "test-token-123")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		t.Fatalf("请求失败: %v", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("期望 400 (缺少 confirm=true), 实际 %d", resp.StatusCode)
	}
}
