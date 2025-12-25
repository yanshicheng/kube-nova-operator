/*
Copyright 2025 IKubeOps By Yanshicheng.
*/

package validator

import (
	"fmt"
	"strings"

	kubenovav1 "github.com/yanshicheng/kube-nova-operator/api/v1"
)

// ValidateKubeNova 验证 KubeNova 配置
func ValidateKubeNova(kn *kubenovav1.KubeNova) error {
	// 验证数据库配置
	if err := validateDatabase(&kn.Spec.Database); err != nil {
		return fmt.Errorf("数据库配置错误: %w", err)
	}

	// 验证缓存配置
	if err := validateCache(&kn.Spec.Cache); err != nil {
		return fmt.Errorf("缓存配置错误: %w", err)
	}

	// 验证存储配置
	if err := validateStorage(&kn.Spec.Storage); err != nil {
		return fmt.Errorf("存储配置错误: %w", err)
	}

	// 验证链路追踪配置（如果启用）
	if kn.IsTelemetryEnabled() {
		if err := kn.Spec.Telemetry.ValidateTelemetryConfig(); err != nil {
			return fmt.Errorf("链路追踪配置错误: %w", err)
		}
	}

	// 验证 JWT 配置
	if err := validateJWT(&kn.Spec.Services.JWT); err != nil {
		return fmt.Errorf("JWT 配置错误: %w", err)
	}

	// 验证 Web 配置
	if err := kn.Spec.Web.ValidateWebConfig(); err != nil {
		return fmt.Errorf("web 配置错误: %w", err)
	}

	return nil
}

// validateDatabase 验证数据库配置
func validateDatabase(db *kubenovav1.DatabaseConfig) error {
	if db.Host == "" {
		return fmt.Errorf("数据库主机地址不能为空")
	}
	if db.Database == "" {
		return fmt.Errorf("数据库名称不能为空")
	}
	if db.User == "" {
		return fmt.Errorf("数据库用户名不能为空")
	}
	if db.Password == "" {
		return fmt.Errorf("数据库密码不能为空")
	}
	if db.Port <= 0 || db.Port > 65535 {
		return fmt.Errorf("数据库端口无效: %d", db.Port)
	}
	return nil
}

// validateCache 验证缓存配置
func validateCache(cache *kubenovav1.CacheConfig) error {
	if cache.Host == "" {
		return fmt.Errorf("redis 主机地址不能为空")
	}
	if cache.Port <= 0 || cache.Port > 65535 {
		return fmt.Errorf("redis 端口无效: %d", cache.Port)
	}
	if cache.Type != "node" && cache.Type != "cluster" {
		return fmt.Errorf("redis 类型必须是 node 或 cluster，当前值: %s", cache.Type)
	}
	return nil
}

// validateStorage 验证存储配置
func validateStorage(storage *kubenovav1.StorageConfig) error {
	if storage.Endpoint == "" {
		return fmt.Errorf("存储端点地址不能为空")
	}
	if storage.AccessKey == "" {
		return fmt.Errorf("存储访问密钥不能为空")
	}
	if storage.SecretKey == "" {
		return fmt.Errorf("存储密钥不能为空")
	}
	if storage.Bucket == "" {
		return fmt.Errorf("存储桶名称不能为空")
	}

	// 验证 TLS 配置
	if storage.TLS != nil && storage.TLS.Enabled {
		if err := storage.TLS.ValidateMinIOTLS(); err != nil {
			return fmt.Errorf("MinIO TLS 配置错误: %w", err)
		}
	}

	return nil
}

// validateJWT 验证 JWT 配置
func validateJWT(jwt *kubenovav1.JWTConfig) error {
	if jwt.AccessSecret == "" {
		return fmt.Errorf("访问令牌密钥不能为空")
	}
	if len(jwt.AccessSecret) < 32 {
		return fmt.Errorf("访问令牌密钥长度至少需要 32 个字符，当前长度: %d", len(jwt.AccessSecret))
	}
	if jwt.RefreshSecret == "" {
		return fmt.Errorf("刷新令牌密钥不能为空")
	}
	if len(jwt.RefreshSecret) < 32 {
		return fmt.Errorf("刷新令牌密钥长度至少需要 32 个字符，当前长度: %d", len(jwt.RefreshSecret))
	}
	return nil
}

// ValidateTLSSecret 验证 TLS Secret 是否存在（运行时验证）
func ValidateTLSSecret(secretName string) error {
	if secretName == "" {
		return fmt.Errorf("TLS Secret 名称不能为空")
	}
	return nil
}

// validateSecretName 验证 Secret 名称格式
func validateSecretName(name string) error {
	if name == "" {
		return fmt.Errorf("secret 名称不能为空")
	}
	if len(name) > 253 {
		return fmt.Errorf("secret 名称过长，最大长度 253，当前长度: %d", len(name))
	}
	if strings.Contains(name, " ") {
		return fmt.Errorf("secret 名称不能包含空格")
	}
	return nil
}
