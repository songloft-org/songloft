package services

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"strconv"
	"sync"

	"mimusic/internal/database"
	"mimusic/internal/models"
)

// ConfigRepository 配置仓储接口（ConfigService 依赖）。
type ConfigRepository interface {
	Get(ctx context.Context, key string) (*models.Config, error)
	Set(ctx context.Context, config *models.Config) error
	Delete(ctx context.Context, key string) error
	List(ctx context.Context, filter *database.ConfigFilter) ([]*models.Config, error)
	Count(ctx context.Context, filter *database.ConfigFilter) (int64, error)
}

// ConfigService 配置服务
type ConfigService struct {
	repo  ConfigRepository
	cache sync.Map // 配置缓存
	mu    sync.RWMutex
}

// NewConfigService 创建配置服务实例
func NewConfigService(repo ConfigRepository) *ConfigService {
	return &ConfigService{
		repo: repo,
	}
}

// GetString 获取字符串配置
func (s *ConfigService) GetString(key, defaultValue string) string {
	// 先从缓存读取
	if value, ok := s.cache.Load(key); ok {
		if strValue, ok := value.(string); ok {
			return strValue
		}
	}

	// 从数据库读取
	ctx := context.Background()
	config, err := s.repo.Get(ctx, key)
	if err != nil {
		slog.Warn("获取配置失败，使用默认值", "key", key, "default", defaultValue, "error", err)
		return defaultValue
	}

	// 存入缓存
	s.cache.Store(key, config.Value)
	return config.Value
}

// GetInt 获取整数配置
func (s *ConfigService) GetInt(key string, defaultValue int) int {
	strValue := s.GetString(key, "")
	if strValue == "" {
		return defaultValue
	}

	intValue, err := strconv.Atoi(strValue)
	if err != nil {
		slog.Warn("配置值转换为整数失败，使用默认值", "key", key, "value", strValue, "default", defaultValue, "error", err)
		return defaultValue
	}

	return intValue
}

// GetBool 获取布尔配置
func (s *ConfigService) GetBool(key string, defaultValue bool) bool {
	strValue := s.GetString(key, "")
	if strValue == "" {
		return defaultValue
	}

	boolValue, err := strconv.ParseBool(strValue)
	if err != nil {
		slog.Warn("配置值转换为布尔值失败，使用默认值", "key", key, "value", strValue, "default", defaultValue, "error", err)
		return defaultValue
	}

	return boolValue
}

// GetJSON 获取 JSON 配置并解析到目标结构体
func (s *ConfigService) GetJSON(key string, target interface{}) error {
	// 先从缓存读取
	if value, ok := s.cache.Load(key); ok {
		if strValue, ok := value.(string); ok {
			if err := json.Unmarshal([]byte(strValue), target); err != nil {
				slog.Error("缓存配置 JSON 解析失败", "key", key, "error", err)
				s.cache.Delete(key) // 删除无效缓存
			} else {
				return nil
			}
		}
	}

	// 从数据库读取
	ctx := context.Background()
	config, err := s.repo.Get(ctx, key)
	if err != nil {
		return fmt.Errorf("获取配置失败: %w", err)
	}

	// 解析 JSON
	if err := json.Unmarshal([]byte(config.Value), target); err != nil {
		return fmt.Errorf("配置 JSON 解析失败: %w", err)
	}

	// 存入缓存
	s.cache.Store(key, config.Value)
	return nil
}

// Set 设置配置
func (s *ConfigService) Set(key, value string) error {
	ctx := context.Background()
	config := &models.Config{
		Key:   key,
		Value: value,
	}

	if err := s.repo.Set(ctx, config); err != nil {
		return fmt.Errorf("设置配置失败: %w", err)
	}

	// 更新缓存
	s.cache.Store(key, value)
	return nil
}

// SetJSON 设置 JSON 配置
func (s *ConfigService) SetJSON(key string, value interface{}) error {
	jsonBytes, err := json.Marshal(value)
	if err != nil {
		return fmt.Errorf("配置 JSON 序列化失败: %w", err)
	}

	return s.Set(key, string(jsonBytes))
}

// ClearCache 清除配置缓存
func (s *ConfigService) ClearCache() {
	s.cache = sync.Map{}
}

// ClearCacheKey 清除指定配置的缓存
func (s *ConfigService) ClearCacheKey(key string) {
	s.cache.Delete(key)
}

// ListConfigs 获取配置列表
func (s *ConfigService) ListConfigs(ctx context.Context, filter *database.ConfigFilter) ([]*models.Config, error) {
	return s.repo.List(ctx, filter)
}

// CountConfigs 获取配置总数
func (s *ConfigService) CountConfigs(ctx context.Context, filter *database.ConfigFilter) (int64, error) {
	return s.repo.Count(ctx, filter)
}

// GetConfig 获取单个配置
func (s *ConfigService) GetConfig(ctx context.Context, key string) (*models.Config, error) {
	return s.repo.Get(ctx, key)
}

// CreateConfig 创建配置
func (s *ConfigService) CreateConfig(ctx context.Context, config *models.Config) error {
	if err := s.repo.Set(ctx, config); err != nil {
		return fmt.Errorf("创建配置失败: %w", err)
	}

	// 清除缓存
	s.ClearCacheKey(config.Key)
	return nil
}

// UpdateConfig 更新配置
func (s *ConfigService) UpdateConfig(ctx context.Context, config *models.Config) error {
	if err := s.repo.Set(ctx, config); err != nil {
		return fmt.Errorf("更新配置失败: %w", err)
	}

	// 清除缓存
	s.ClearCacheKey(config.Key)
	return nil
}

// DeleteConfig 删除配置
func (s *ConfigService) DeleteConfig(ctx context.Context, key string) error {
	if err := s.repo.Delete(ctx, key); err != nil {
		return fmt.Errorf("删除配置失败: %w", err)
	}

	// 清除缓存
	s.ClearCacheKey(key)
	return nil
}
