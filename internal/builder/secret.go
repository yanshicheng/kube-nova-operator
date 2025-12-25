/*
Copyright 2025 Kube-nova By YanShicheng.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package builder

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubenovav1 "github.com/yanshicheng/kube-nova-operator/api/v1"
)

// BuildSecret 构建 Secret
func BuildSecret(kn *kubenovav1.KubeNova, namespace string, nodeIP string, nodePort int32) *corev1.Secret {
	data := make(map[string][]byte)

	// 全局配置
	data["DEFAULT_TIMEOUT"] = []byte(fmt.Sprintf("%d", kn.Spec.Services.GlobalTimeout))

	// MySQL 数据库配置
	data["MYSQL_HOST"] = []byte(kn.Spec.Database.Host)
	data["MYSQL_PORT"] = []byte(fmt.Sprintf("%d", kn.Spec.Database.Port))
	data["MYSQL_DATABASE"] = []byte(kn.Spec.Database.Database)
	data["MYSQL_USER"] = []byte(kn.Spec.Database.User)
	data["MYSQL_PASSWORD"] = []byte(kn.Spec.Database.Password)
	data["MYSQL_MAX_OPEN_CONNS"] = []byte(fmt.Sprintf("%d", kn.Spec.Database.GetMaxOpenConns()))
	data["MYSQL_MAX_IDLE_CONNS"] = []byte(fmt.Sprintf("%d", kn.Spec.Database.GetMaxIdleConns()))
	data["MYSQL_CONN_MAX_LIFETIME"] = []byte(kn.Spec.Database.GetConnMaxLifetime())

	// Redis 缓存配置
	data["REDIS_HOST"] = []byte(kn.Spec.Cache.Host)
	data["REDIS_PORT"] = []byte(fmt.Sprintf("%d", kn.Spec.Cache.Port))
	data["REDIS_PASSWORD"] = []byte(kn.Spec.Cache.GetCachePassword())
	data["REDIS_TYPE"] = []byte(kn.Spec.Cache.Type)
	data["REDIS_TLS"] = []byte(fmt.Sprintf("%t", kn.Spec.Cache.TLS))
	data["REDIS_NONBLOCK"] = []byte(fmt.Sprintf("%t", kn.Spec.Cache.NonBlock))
	data["REDIS_PING_TIMEOUT"] = []byte(kn.Spec.Cache.PingTimeout)

	// MinIO 对象存储配置
	// MINIO_ENDPOINT: SDK 使用的纯地址:端口（不带协议和路径）
	data["MINIO_ENDPOINT"] = []byte(kn.Spec.Storage.Endpoint)
	data["MINIO_ACCESS_KEY"] = []byte(kn.Spec.Storage.AccessKey)
	data["MINIO_SECRET_KEY"] = []byte(kn.Spec.Storage.SecretKey)
	data["MINIO_BUCKET"] = []byte(kn.Spec.Storage.Bucket)

	// MINIO_USE_SSL: SDK 是否使用 SSL（取决于 MinIO 是否启用 TLS）
	useSSL := kn.Spec.Storage.TLS != nil && kn.Spec.Storage.TLS.Enabled
	data["MINIO_USE_SSL"] = []byte(fmt.Sprintf("%t", useSSL))

	// MinIO TLS 证书配置
	if useSSL {
		// 证书文件路径，portal-rpc 会挂载到这个位置
		data["MINIO_CA_FILE"] = []byte("/app/etc/minio-certs/public.crt")
		data["MINIO_CA_KEY"] = []byte("/app/etc/minio-certs/private.key")
	} else {
		data["MINIO_CA_FILE"] = []byte("")
		data["MINIO_CA_KEY"] = []byte("")
	}

	// MINIO_ENDPOINT_PROXY: 前端访问使用的完整 URL
	// - 未启用代理：http(s)://实际地址（是否 https 看 TLS）
	// - 启用代理：代理地址（如 http://example.com/storage）
	var endpointProxy string
	if kn.Spec.Storage.EndpointProxy != "" {
		endpointProxy = kn.Spec.Storage.EndpointProxy
	} else {
		if kn.IsMinIOProxyEnabled() {
			endpointProxy = kn.GetMinIOEndpointForBackendWithNodeInfo(nodeIP, nodePort)
		} else {
			// 未启用代理：使用实际 MinIO 地址（带协议）
			protocol := "http"
			if useSSL {
				protocol = "https"
			}
			endpointProxy = fmt.Sprintf("%s://%s", protocol, kn.Spec.Storage.Endpoint)
		}
	}

	data["MINIO_ENDPOINT_PROXY"] = []byte(endpointProxy)

	// JWT 认证配置
	data["JWT_ACCESS_SECRET"] = []byte(kn.Spec.Services.JWT.AccessSecret)
	data["JWT_ACCESS_EXPIRE"] = []byte(fmt.Sprintf("%d", kn.Spec.Services.JWT.AccessExpire))
	data["JWT_REFRESH_SECRET"] = []byte(kn.Spec.Services.JWT.RefreshSecret)
	data["JWT_REFRESH_EXPIRE"] = []byte(fmt.Sprintf("%d", kn.Spec.Services.JWT.RefreshExpire))
	data["JWT_REFRESH_AFTER"] = []byte(fmt.Sprintf("%d", kn.Spec.Services.JWT.RefreshAfter))

	// Webhook 配置
	if kn.Spec.Services.WebhookToken != "" {
		data["ALERTMANAGER_WEBHOOK_TOKEN"] = []byte(kn.Spec.Services.WebhookToken)
	} else {
		data["ALERTMANAGER_WEBHOOK_TOKEN"] = []byte("")
	}

	// Jaeger 链路追踪配置
	if kn.IsTelemetryEnabled() {
		data["JAEGER_ENDPOINT"] = []byte(kn.Spec.Telemetry.JaegerEndpoint)
		data["TELEMETRY_SAMPLER"] = []byte(kn.Spec.Telemetry.Sampler)
		data["TELEMETRY_BATCHER"] = []byte(kn.Spec.Telemetry.Batcher)
	} else {
		data["JAEGER_ENDPOINT"] = []byte("")
		data["TELEMETRY_SAMPLER"] = []byte("0")
		data["TELEMETRY_BATCHER"] = []byte("jaeger")
	}

	// Portal 门户配置
	if kn.Spec.Services.Portal != nil {
		data["PORTAL_NAME"] = []byte(kn.Spec.Services.Portal.Name)
		data["PORTAL_URL"] = []byte(kn.Spec.Services.Portal.URL)
		data["DEMO_MODE"] = []byte(fmt.Sprintf("%t", kn.Spec.Services.Portal.DemoMode))
	} else {
		data["PORTAL_NAME"] = []byte("Kube-Nova 云原生平台")
		data["PORTAL_URL"] = []byte("")
		data["DEMO_MODE"] = []byte("false")
	}

	// 注入镜像配置
	data["INJECT_IMAGE"] = []byte(kn.Spec.Services.InjectImage)

	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-nova-secret",
			Namespace: namespace,
			Labels: map[string]string{
				"app.kubernetes.io/name":       "kube-nova",
				"app.kubernetes.io/instance":   kn.Name,
				"app.kubernetes.io/managed-by": "kube-nova-operator",
			},
		},
		Type: corev1.SecretTypeOpaque,
		Data: data,
	}
}
