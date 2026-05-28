package services

import (
	"context"
	"testing"

	"songloft/internal/database/testutil"
	"songloft/internal/models"
)

// newTestConfigRepo 启动 :memory: SQLite，返回 ConfigRepository。
func newTestConfigRepo(t *testing.T) interface {
	ConfigRepository
} {
	t.Helper()
	mdb := testutil.OpenMemoryDB(t)
	return mdb.ConfigRepository()
}

// TestConfigServiceGetString 测试获取字符串配置
func TestConfigServiceGetString(t *testing.T) {
	repo := newTestConfigRepo(t)
	service := NewConfigService(repo)
	ctx := context.Background()

	// 设置配置
	if err := repo.Set(ctx, &models.Config{Key: "test_key", Value: "test_value"}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// 测试获取
	value := service.GetString("test_key", "default")
	if value != "test_value" {
		t.Errorf("GetString() = %v, want %v", value, "test_value")
	}

	// 测试获取不存在的配置（返回默认值）
	value = service.GetString("non_existent", "default")
	if value != "default" {
		t.Errorf("GetString() for non-existent key = %v, want %v", value, "default")
	}
}

// TestConfigServiceGetInt 测试获取整数配置
func TestConfigServiceGetInt(t *testing.T) {
	repo := newTestConfigRepo(t)
	service := NewConfigService(repo)
	ctx := context.Background()

	if err := repo.Set(ctx, &models.Config{Key: "port", Value: "8080"}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	value := service.GetInt("port", 3000)
	if value != 8080 {
		t.Errorf("GetInt() = %v, want %v", value, 8080)
	}

	value = service.GetInt("non_existent", 3000)
	if value != 3000 {
		t.Errorf("GetInt() for non-existent key = %v, want %v", value, 3000)
	}

	if err := repo.Set(ctx, &models.Config{Key: "invalid", Value: "not_a_number"}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	value = service.GetInt("invalid", 3000)
	if value != 3000 {
		t.Errorf("GetInt() for invalid value = %v, want %v", value, 3000)
	}
}

// TestConfigServiceGetBool 测试获取布尔配置
func TestConfigServiceGetBool(t *testing.T) {
	repo := newTestConfigRepo(t)
	service := NewConfigService(repo)
	ctx := context.Background()

	if err := repo.Set(ctx, &models.Config{Key: "enabled", Value: "true"}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	value := service.GetBool("enabled", false)
	if value != true {
		t.Errorf("GetBool() = %v, want %v", value, true)
	}

	value = service.GetBool("non_existent", false)
	if value != false {
		t.Errorf("GetBool() for non-existent key = %v, want %v", value, false)
	}
}

// TestConfigServiceGetJSON 测试获取 JSON 配置
func TestConfigServiceGetJSON(t *testing.T) {
	repo := newTestConfigRepo(t)
	service := NewConfigService(repo)
	ctx := context.Background()

	if err := repo.Set(ctx, &models.Config{
		Key:   "settings",
		Value: `{"name":"test","value":123}`,
	}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	var result struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	if err := service.GetJSON("settings", &result); err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}
	if result.Name != "test" || result.Value != 123 {
		t.Errorf("GetJSON() result = %+v, want {Name:test Value:123}", result)
	}

	if err := service.GetJSON("non_existent", &result); err == nil {
		t.Error("GetJSON() for non-existent key should return error")
	}
}

// TestConfigServiceSet 测试设置配置
func TestConfigServiceSet(t *testing.T) {
	repo := newTestConfigRepo(t)
	service := NewConfigService(repo)

	if err := service.Set("test_key", "test_value"); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	value := service.GetString("test_key", "")
	if value != "test_value" {
		t.Errorf("Set() value = %v, want %v", value, "test_value")
	}
}

// TestConfigServiceSetJSON 测试设置 JSON 配置
func TestConfigServiceSetJSON(t *testing.T) {
	repo := newTestConfigRepo(t)
	service := NewConfigService(repo)

	data := struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}{
		Name:  "test",
		Value: 123,
	}
	if err := service.SetJSON("settings", data); err != nil {
		t.Fatalf("SetJSON() error = %v", err)
	}

	var result struct {
		Name  string `json:"name"`
		Value int    `json:"value"`
	}
	if err := service.GetJSON("settings", &result); err != nil {
		t.Fatalf("GetJSON() error = %v", err)
	}
	if result.Name != "test" || result.Value != 123 {
		t.Errorf("SetJSON() result = %+v, want {Name:test Value:123}", result)
	}
}

// TestConfigServiceClearCache 测试清除缓存
func TestConfigServiceClearCache(t *testing.T) {
	repo := newTestConfigRepo(t)
	service := NewConfigService(repo)
	ctx := context.Background()

	// 写入初值并触发缓存
	if err := repo.Set(ctx, &models.Config{Key: "test", Value: "value1"}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	_ = service.GetString("test", "")

	// 直接绕过 service 改写 DB
	if err := repo.Set(ctx, &models.Config{Key: "test", Value: "value2"}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	// 清除缓存后,应该读到新值
	service.ClearCache()
	got := service.GetString("test", "")
	if got != "value2" {
		t.Errorf("GetString() after ClearCache() = %v, want %v", got, "value2")
	}
}

// TestConfigServiceClearCacheKey 测试清除指定键的缓存
func TestConfigServiceClearCacheKey(t *testing.T) {
	repo := newTestConfigRepo(t)
	service := NewConfigService(repo)
	ctx := context.Background()

	if err := repo.Set(ctx, &models.Config{Key: "test", Value: "value1"}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}
	_ = service.GetString("test", "")

	service.ClearCacheKey("test")

	if err := repo.Set(ctx, &models.Config{Key: "test", Value: "value2"}); err != nil {
		t.Fatalf("Set() error = %v", err)
	}

	got := service.GetString("test", "")
	if got != "value2" {
		t.Errorf("GetString() after ClearCacheKey() = %v, want %v", got, "value2")
	}
}
