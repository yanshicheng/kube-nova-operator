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
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubenovav1 "github.com/yanshicheng/kube-nova-operator/api/v1"
)

// BuildAllConfigMaps 构建所有 ConfigMap
func BuildAllConfigMaps(kn *kubenovav1.KubeNova, namespace string) []*corev1.ConfigMap {
	configMaps := []*corev1.ConfigMap{
		buildPortalAPIConfigMap(kn, namespace),
		buildPortalRPCConfigMap(kn, namespace),
		buildManagerAPIConfigMap(kn, namespace),
		buildManagerRPCConfigMap(kn, namespace),
		buildWorkloadAPIConfigMap(kn, namespace),
		buildConsoleAPIConfigMap(kn, namespace),
		buildConsoleRPCConfigMap(kn, namespace),
	}
	return configMaps
}

// buildPortalAPIConfigMap 构建 Portal API ConfigMap
func buildPortalAPIConfigMap(kn *kubenovav1.KubeNova, namespace string) *corev1.ConfigMap {
	config := `Name: portal-api
Host: 0.0.0.0
Port: 8810
Mode: pro
Timeout: ${DEFAULT_TIMEOUT}
MaxBytes: 10485760

DevServer:
  Enabled: true
  Port: 9999
  HealthPath: "/healthz"
  MetricsPath: "/metrics"
  EnableMetrics: true

Telemetry:
  Name: portal-api
  Endpoint: ${JAEGER_ENDPOINT}
  Sampler: ${TELEMETRY_SAMPLER}
  Batcher: ${TELEMETRY_BATCHER}

Log:
  ServiceName: portal-api
  Mode: console
  Encoding: plain
  TimeFormat: "2006-01-02 15:04:05"
  Path: logs
  Level: info
  MaxContentLength: 1024
  Compress: false
  Stat: true
  KeepDays: 7
  StackCooldownMillis: 100
  MaxBackups: 0
  MaxSize: 0
  Rotation: daily

Cache:
  Host: ${REDIS_HOST}:${REDIS_PORT}
  Type: ${REDIS_TYPE}
  Pass: ${REDIS_PASSWORD}
  Tls: ${REDIS_TLS}
  NonBlock: ${REDIS_NONBLOCK}
  PingTimeout: ${REDIS_PING_TIMEOUT}

PortalRpc:
  Target: k8s://${POD_NAMESPACE}/portal-rpc:30010
  Optional: true
  NonBlock: true
  Timeout: ${DEFAULT_TIMEOUT}
`

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "portal-api-config",
			Namespace: namespace,
			Labels:    getCommonLabels(kn),
		},
		Data: map[string]string{
			"config.yaml": config,
		},
	}
}

// buildPortalRPCConfigMap 构建 Portal RPC ConfigMap
func buildPortalRPCConfigMap(kn *kubenovav1.KubeNova, namespace string) *corev1.ConfigMap {
	// 获取 portal-rpc 的有效 Leader Election 配置
	leaderElectionConfig := getEffectiveLeaderElectionForService(kn, kn.Spec.Services.PortalRPC, "portal-rpc", namespace)

	config := `Name: portal.rpc
ListenOn: 0.0.0.0:30010
Mode: pro
Timeout: ${DEFAULT_TIMEOUT}
DemoMode: ${DEMO_MODE}
PortalName: ${PORTAL_NAME}
PortalUrl: ${PORTAL_URL}

DevServer:
  Enabled: true
  Port: 9999
  HealthPath: "/healthz"
  MetricsPath: "/metrics"
  EnableMetrics: true

Telemetry:
  Name: portal-rpc
  Endpoint: ${JAEGER_ENDPOINT}
  Sampler: ${TELEMETRY_SAMPLER}
  Batcher: ${TELEMETRY_BATCHER}

Log:
  ServiceName: portal-rpc
  Mode: console
  Encoding: plain
  TimeFormat: "2006-01-02 15:04:05"
  Path: logs
  Level: info
  MaxContentLength: 1024
  Compress: false
  Stat: true
  KeepDays: 7
  StackCooldownMillis: 100
  MaxBackups: 0
  MaxSize: 0
  Rotation: daily

Cache:
  Host: ${REDIS_HOST}:${REDIS_PORT}
  Type: ${REDIS_TYPE}
  Pass: ${REDIS_PASSWORD}
  Tls: ${REDIS_TLS}
  NonBlock: ${REDIS_NONBLOCK}
  PingTimeout: ${REDIS_PING_TIMEOUT}

Mysql:
  DataSource: ${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(${MYSQL_HOST}:${MYSQL_PORT})/${MYSQL_DATABASE}?charset=utf8mb4&parseTime=True&loc=Local&timeout=30s
  MaxOpenConns: ${MYSQL_MAX_OPEN_CONNS}
  MaxIdleConns: ${MYSQL_MAX_IDLE_CONNS}
  ConnMaxLifetime: ${MYSQL_CONN_MAX_LIFETIME}

DBCache:
  - Host: ${REDIS_HOST}:${REDIS_PORT}
    Type: ${REDIS_TYPE}
    Pass: ${REDIS_PASSWORD}
    Tls: ${REDIS_TLS}
    NonBlock: ${REDIS_NONBLOCK}
    PingTimeout: ${REDIS_PING_TIMEOUT}

StorageConf:
  Provider: minio
  Endpoints: ["${MINIO_ENDPOINT}"]
  EndpointProxy: ${MINIO_ENDPOINT_PROXY}
  AccessKey: ${MINIO_ACCESS_KEY}
  AccessSecret: ${MINIO_SECRET_KEY}
  BucketName: ${MINIO_BUCKET}
  UseTLS: ${MINIO_USE_SSL}
  CAFile: ${MINIO_CA_FILE}
  CAKey: ${MINIO_CA_KEY}

AuthConfig:
  AccessSecret: ${JWT_ACCESS_SECRET}
  AccessExpire: ${JWT_ACCESS_EXPIRE}
  RefreshSecret: ${JWT_REFRESH_SECRET}
  RefreshExpire: ${JWT_REFRESH_EXPIRE}
  RefreshAfter: ${JWT_REFRESH_AFTER}

Aggregator:
  Enabled: true
  MaxBufferSize: 1000
  GlobalBufferWindow: 30s
  SeverityWindows:
    Critical: 0s
    Warning: 1m
    Info: 1m
    Default: 1m
` + leaderElectionConfig

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "portal-rpc-config",
			Namespace: namespace,
			Labels:    getCommonLabels(kn),
		},
		Data: map[string]string{
			"config.yaml": config,
		},
	}
}

// buildManagerAPIConfigMap 构建 Manager API ConfigMap
func buildManagerAPIConfigMap(kn *kubenovav1.KubeNova, namespace string) *corev1.ConfigMap {
	config := `Name: manager-api
Host: 0.0.0.0
Port: 8811
Mode: pro
Timeout: ${DEFAULT_TIMEOUT}
MaxBytes: 10485760

Webhook:
  Token: ${WEBHOOK_TOKEN}

DevServer:
  Enabled: true
  Port: 9999
  HealthPath: "/healthz"
  MetricsPath: "/metrics"
  EnableMetrics: true

Telemetry:
  Name: manager-api
  Endpoint: ${JAEGER_ENDPOINT}
  Sampler: ${TELEMETRY_SAMPLER}
  Batcher: ${TELEMETRY_BATCHER}

Log:
  ServiceName: manager-api
  Mode: console
  Encoding: plain
  TimeFormat: "2006-01-02 15:04:05"
  Path: logs
  Level: info
  MaxContentLength: 1024
  Compress: false
  Stat: true
  KeepDays: 7
  StackCooldownMillis: 100
  MaxBackups: 0
  MaxSize: 0
  Rotation: daily

Cache:
  Host: ${REDIS_HOST}:${REDIS_PORT}
  Type: ${REDIS_TYPE}
  Pass: ${REDIS_PASSWORD}
  Tls: ${REDIS_TLS}
  NonBlock: ${REDIS_NONBLOCK}
  PingTimeout: ${REDIS_PING_TIMEOUT}

ManagerRpc:
  Target: k8s://${POD_NAMESPACE}/manager-rpc:30011
  Optional: true
  NonBlock: true
  Timeout: ${DEFAULT_TIMEOUT}

PortalRpc:
  Target: k8s://${POD_NAMESPACE}/portal-rpc:30010
  Optional: true
  NonBlock: true
  Timeout: ${DEFAULT_TIMEOUT}
`

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "manager-api-config",
			Namespace: namespace,
			Labels:    getCommonLabels(kn),
		},
		Data: map[string]string{
			"config.yaml": config,
		},
	}
}

// buildManagerRPCConfigMap 构建 Manager RPC ConfigMap
func buildManagerRPCConfigMap(kn *kubenovav1.KubeNova, namespace string) *corev1.ConfigMap {
	// 获取 manager-rpc 的有效 Leader Election 配置
	leaderElectionConfig := getEffectiveLeaderElectionForService(kn, kn.Spec.Services.ManagerRPC, "manager-rpc", namespace)

	config := `Name: manager.rpc
ListenOn: 0.0.0.0:30011
Mode: pro
Timeout: ${DEFAULT_TIMEOUT}

DevServer:
  Enabled: true
  Port: 9999
  HealthPath: "/healthz"
  MetricsPath: "/metrics"
  EnableMetrics: true

Telemetry:
  Name: manager-rpc
  Endpoint: ${JAEGER_ENDPOINT}
  Sampler: ${TELEMETRY_SAMPLER}
  Batcher: ${TELEMETRY_BATCHER}

Log:
  ServiceName: manager-rpc
  Mode: console
  Encoding: plain
  TimeFormat: "2006-01-02 15:04:05"
  Path: logs
  Level: info
  MaxContentLength: 1024
  Compress: false
  Stat: true
  KeepDays: 7
  StackCooldownMillis: 100
  MaxBackups: 0
  MaxSize: 0
  Rotation: daily

Cache:
  Host: ${REDIS_HOST}:${REDIS_PORT}
  Type: ${REDIS_TYPE}
  Pass: ${REDIS_PASSWORD}
  Tls: ${REDIS_TLS}
  NonBlock: ${REDIS_NONBLOCK}
  PingTimeout: ${REDIS_PING_TIMEOUT}

Mysql:
  DataSource: ${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(${MYSQL_HOST}:${MYSQL_PORT})/${MYSQL_DATABASE}?charset=utf8mb4&parseTime=True&loc=Local&timeout=30s
  MaxOpenConns: ${MYSQL_MAX_OPEN_CONNS}
  MaxIdleConns: ${MYSQL_MAX_IDLE_CONNS}
  ConnMaxLifetime: ${MYSQL_CONN_MAX_LIFETIME}

DBCache:
  - Host: ${REDIS_HOST}:${REDIS_PORT}
    Type: ${REDIS_TYPE}
    Pass: ${REDIS_PASSWORD}
    Tls: ${REDIS_TLS}
    NonBlock: ${REDIS_NONBLOCK}
    PingTimeout: ${REDIS_PING_TIMEOUT}

PortalRpc:
  Target: k8s://${POD_NAMESPACE}/portal-rpc:30010
  Optional: true
  NonBlock: true
  Timeout: ${DEFAULT_TIMEOUT}
` + leaderElectionConfig

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "manager-rpc-config",
			Namespace: namespace,
			Labels:    getCommonLabels(kn),
		},
		Data: map[string]string{
			"config.yaml": config,
		},
	}
}

// buildWorkloadAPIConfigMap 构建 Workload API ConfigMap
func buildWorkloadAPIConfigMap(kn *kubenovav1.KubeNova, namespace string) *corev1.ConfigMap {
	config := `Name: workload-api
Host: 0.0.0.0
Port: 8812
Mode: pro
Timeout: ${DEFAULT_TIMEOUT}
InjectImage: ${INJECT_IMAGE}

DevServer:
  Enabled: true
  Port: 9999
  HealthPath: "/healthz"
  MetricsPath: "/metrics"
  EnableMetrics: true

Telemetry:
  Name: workload-api
  Endpoint: ${JAEGER_ENDPOINT}
  Sampler: ${TELEMETRY_SAMPLER}
  Batcher: ${TELEMETRY_BATCHER}

Log:
  ServiceName: workload-api
  Mode: console
  Encoding: plain
  TimeFormat: "2006-01-02 15:04:05"
  Path: logs
  Level: info
  MaxContentLength: 1024
  Compress: false
  Stat: true
  KeepDays: 7
  StackCooldownMillis: 100
  MaxBackups: 0
  MaxSize: 0
  Rotation: daily

Cache:
  Host: ${REDIS_HOST}:${REDIS_PORT}
  Type: ${REDIS_TYPE}
  Pass: ${REDIS_PASSWORD}
  Tls: ${REDIS_TLS}
  NonBlock: ${REDIS_NONBLOCK}
  PingTimeout: ${REDIS_PING_TIMEOUT}

ManagerRpc:
  Target: k8s://${POD_NAMESPACE}/manager-rpc:30011
  Optional: true
  NonBlock: true
  Timeout: ${DEFAULT_TIMEOUT}

PortalRpc:
  Target: k8s://${POD_NAMESPACE}/portal-rpc:30010
  Optional: true
  NonBlock: true
  Timeout: ${DEFAULT_TIMEOUT}
`

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "workload-api-config",
			Namespace: namespace,
			Labels:    getCommonLabels(kn),
		},
		Data: map[string]string{
			"config.yaml": config,
		},
	}
}

// buildConsoleAPIConfigMap 构建 Console API ConfigMap
func buildConsoleAPIConfigMap(kn *kubenovav1.KubeNova, namespace string) *corev1.ConfigMap {
	config := `Name: console-api
Host: 0.0.0.0
Port: 8818
Mode: pro
Timeout: ${DEFAULT_TIMEOUT}
MaxBytes: 5048576000
LocalCacheDir: /app/cache

DevServer:
  Enabled: true
  Port: 9999
  HealthPath: "/healthz"
  MetricsPath: "/metrics"
  EnableMetrics: true

Telemetry:
  Name: console-api
  Endpoint: ${JAEGER_ENDPOINT}
  Sampler: ${TELEMETRY_SAMPLER}
  Batcher: ${TELEMETRY_BATCHER}

Log:
  ServiceName: console-api
  Mode: console
  Encoding: plain
  TimeFormat: "2006-01-02 15:04:05"
  Path: logs
  Level: info
  MaxContentLength: 1024
  Compress: false
  Stat: true
  KeepDays: 7
  StackCooldownMillis: 100
  MaxBackups: 0
  MaxSize: 0
  Rotation: daily

Cache:
  Host: ${REDIS_HOST}:${REDIS_PORT}
  Type: ${REDIS_TYPE}
  Pass: ${REDIS_PASSWORD}
  Tls: ${REDIS_TLS}
  NonBlock: ${REDIS_NONBLOCK}
  PingTimeout: ${REDIS_PING_TIMEOUT}

ManagerRpc:
  Target: k8s://${POD_NAMESPACE}/manager-rpc:30011
  Optional: true
  NonBlock: true
  Timeout: ${DEFAULT_TIMEOUT}

ConsoleRpc:
  Target: k8s://${POD_NAMESPACE}/console-rpc:30018
  Optional: true
  NonBlock: true
  Timeout: ${DEFAULT_TIMEOUT}

PortalRpc:
  Target: k8s://${POD_NAMESPACE}/portal-rpc:30010
  Optional: true
  NonBlock: true
  Timeout: ${DEFAULT_TIMEOUT}
`

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "console-api-config",
			Namespace: namespace,
			Labels:    getCommonLabels(kn),
		},
		Data: map[string]string{
			"config.yaml": config,
		},
	}
}

// buildConsoleRPCConfigMap 构建 Console RPC ConfigMap
func buildConsoleRPCConfigMap(kn *kubenovav1.KubeNova, namespace string) *corev1.ConfigMap {
	config := `Name: console.rpc
ListenOn: 0.0.0.0:30018
Mode: pro
Timeout: ${DEFAULT_TIMEOUT}

DevServer:
  Enabled: true
  Port: 9999
  HealthPath: "/healthz"
  MetricsPath: "/metrics"
  EnableMetrics: true

Telemetry:
  Name: console-rpc
  Endpoint: ${JAEGER_ENDPOINT}
  Sampler: ${TELEMETRY_SAMPLER}
  Batcher: ${TELEMETRY_BATCHER}

Log:
  ServiceName: console-rpc
  Mode: console
  Encoding: plain
  TimeFormat: "2006-01-02 15:04:05"
  Path: logs
  Level: info
  MaxContentLength: 1024
  Compress: false
  Stat: true
  KeepDays: 7
  StackCooldownMillis: 100
  MaxBackups: 0
  MaxSize: 0
  Rotation: daily

Cache:
  Host: ${REDIS_HOST}:${REDIS_PORT}
  Type: ${REDIS_TYPE}
  Pass: ${REDIS_PASSWORD}
  Tls: ${REDIS_TLS}
  NonBlock: ${REDIS_NONBLOCK}
  PingTimeout: ${REDIS_PING_TIMEOUT}

Mysql:
  DataSource: ${MYSQL_USER}:${MYSQL_PASSWORD}@tcp(${MYSQL_HOST}:${MYSQL_PORT})/${MYSQL_DATABASE}?charset=utf8mb4&parseTime=True&loc=Local&timeout=30s
  MaxOpenConns: ${MYSQL_MAX_OPEN_CONNS}
  MaxIdleConns: ${MYSQL_MAX_IDLE_CONNS}
  ConnMaxLifetime: ${MYSQL_CONN_MAX_LIFETIME}

DBCache:
  - Host: ${REDIS_HOST}:${REDIS_PORT}
    Type: ${REDIS_TYPE}
    Pass: ${REDIS_PASSWORD}
    Tls: ${REDIS_TLS}
    NonBlock: ${REDIS_NONBLOCK}
    PingTimeout: ${REDIS_PING_TIMEOUT}

ManagerRpc:
  Target: k8s://${POD_NAMESPACE}/manager-rpc:30011
  Optional: true
  NonBlock: true
  Timeout: ${DEFAULT_TIMEOUT}
`

	return &corev1.ConfigMap{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "console-rpc-config",
			Namespace: namespace,
			Labels:    getCommonLabels(kn),
		},
		Data: map[string]string{
			"config.yaml": config,
		},
	}
}

// getCommonLabels 获取公共标签
func getCommonLabels(kn *kubenovav1.KubeNova) map[string]string {
	return map[string]string{
		"app.ikubeops.com/name":       "kube-nova",
		"app.ikubeops.com/instance":   kn.Name,
		"app.ikubeops.com/managed-by": "kube-nova-operator",
	}
}

// getEffectiveLeaderElectionForService 获取服务的有效 Leader Election 配置并生成 YAML 配置
// 优先级：服务级别 > 全局级别
func getEffectiveLeaderElectionForService(kn *kubenovav1.KubeNova, serviceConfig *kubenovav1.ServiceConfig, serviceName, namespace string) string {
	var leConfig *kubenovav1.LeaderElectionConfig

	// 优先使用服务级别配置
	if serviceConfig != nil && serviceConfig.LeaderElection != nil {
		leConfig = serviceConfig.LeaderElection
	} else if kn.Spec.Services.LeaderElection != nil {
		// 否则使用全局配置
		leConfig = kn.Spec.Services.LeaderElection
	}

	// 如果没有配置或未启用，返回空字符串
	if leConfig == nil || !leConfig.Enabled {
		return ""
	}

	// 生成 Leader Election 配置 YAML
	// 注意：LeaseNamespace 直接使用 KubeNova 资源所在的 namespace，无需用户手动指定
	leaseName := leConfig.GetLeaseName(serviceName + "-leader")
	leaseDuration := leConfig.GetLeaseDuration()
	renewDeadline := leConfig.GetRenewDeadline()
	retryPeriod := leConfig.GetRetryPeriod()

	return `
LeaderElection:
  Enabled: true
  LeaseName: "` + leaseName + `"
  LeaseNamespace: "` + namespace + `"
  LeaseDuration: ` + leaseDuration + `
  RenewDeadline: ` + renewDeadline + `
  RetryPeriod: ` + retryPeriod + `
`
}
