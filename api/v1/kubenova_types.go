/*
Copyright 2025 IKubeOps By Yanshicheng.
*/

package v1

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +kubebuilder:validation:Enum=all;k8s;devops
type DeploymentMode string

// KubeNovaSpec 定义了 KubeNova 的期望状态
type KubeNovaSpec struct {
	// ImageRegistry 全局镜像仓库配置
	// +optional
	ImageRegistry *ImageRegistryConfig `json:"imageRegistry,omitempty"`

	// DeploymentMode 部署模式：all(默认，全部部署)、k8s(K8s管理)、devops(DevOps)
	// +kubebuilder:default=all
	DeploymentMode DeploymentMode `json:"deploymentMode,omitempty"`

	// Database 数据库配置(MySQL)
	// +kubebuilder:validation:Required
	Database DatabaseConfig `json:"database"`

	// Cache 缓存配置(Redis)
	// +kubebuilder:validation:Required
	Cache CacheConfig `json:"cache"`

	// Storage 对象存储配置(MinIO/S3)
	// +kubebuilder:validation:Required
	Storage StorageConfig `json:"storage"`

	// Mongo MongoDB 配置(devops 服务使用)
	// 部署模式为 devops 或 all 时必填
	// +optional
	Mongo *MongoConfig `json:"mongo,omitempty"`

	// Telemetry 链路追踪配置(Jaeger)
	// 可选，如果不配置则不启用链路追踪
	// +optional
	Telemetry *TelemetryConfig `json:"telemetry,omitempty"`

	// Services 后端服务配置
	// +kubebuilder:validation:Required
	Services ServicesConfig `json:"services"`

	// Web 前端配置
	// +kubebuilder:validation:Required
	Web WebConfig `json:"web"`
}

// ImageRegistryConfig 全局镜像仓库配置
type ImageRegistryConfig struct {
	// Registry 镜像仓库地址，例如：registry.cn-hangzhou.aliyuncs.com
	// +kubebuilder:default="registry.cn-hangzhou.aliyuncs.com"
	Registry string `json:"registry,omitempty"`

	// Organization 组织/项目名称
	// +kubebuilder:default="kube-nova"
	Organization string `json:"organization,omitempty"`

	// Tag 默认镜像标签
	// +kubebuilder:default="latest"
	Tag string `json:"tag,omitempty"`

	// PullPolicy 镜像拉取策略
	// +kubebuilder:default=Always
	// +kubebuilder:validation:Enum=Always;IfNotPresent;Never
	PullPolicy corev1.PullPolicy `json:"pullPolicy,omitempty"`

	// PullSecrets 镜像拉取密钥列表(Secret 名称)
	// +optional
	PullSecrets []string `json:"pullSecrets,omitempty"`
}

// MongoConfig MongoDB 配置(devops 服务使用)
type MongoConfig struct {
	// Url MongoDB 连接字符串
	// 例如：mongodb://root:12345678@mongo:27017/?authSource=admin
	// +kubebuilder:validation:Required
	Url string `json:"url"`

	// Db 数据库名称
	// +kubebuilder:default="kube_nova_devops"
	Db string `json:"db,omitempty"`
}

// GetMongoUrl 获取 MongoDB 连接字符串
func (m *MongoConfig) GetMongoUrl() string {
	if m == nil {
		return ""
	}
	return m.Url
}

// GetMongoDb 获取 MongoDB 数据库名称
func (m *MongoConfig) GetMongoDb() string {
	if m == nil || m.Db == "" {
		return "kube_nova_devops"
	}
	return m.Db
}

// DatabaseConfig MySQL 数据库配置
type DatabaseConfig struct {
	// Host 数据库主机地址，例如：mysql.default.svc.cluster.local
	// +kubebuilder:validation:Required
	Host string `json:"host"`

	// Port 数据库端口
	// +kubebuilder:default=3306
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// Database 数据库名称
	// +kubebuilder:validation:Required
	Database string `json:"database"`

	// User 数据库用户名
	// +kubebuilder:validation:Required
	User string `json:"user"`

	// Password 数据库密码(明文，Operator 会自动存储到 Secret)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	Password string `json:"password"`

	// MaxOpenConns 最大打开连接数
	// +kubebuilder:default=100
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=1000
	// +optional
	MaxOpenConns int32 `json:"maxOpenConns,omitempty"`

	// MaxIdleConns 最大空闲连接数
	// +kubebuilder:default=50
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=500
	// +optional
	MaxIdleConns int32 `json:"maxIdleConns,omitempty"`

	// ConnMaxLifetime 连接最大生命周期(例如：30m, 1h)
	// +kubebuilder:default="30m"
	// +optional
	ConnMaxLifetime string `json:"connMaxLifetime,omitempty"`
}

// CacheConfig Redis 缓存配置
type CacheConfig struct {
	// Host Redis 主机地址，例如：redis.default.svc.cluster.local
	// +kubebuilder:validation:Required
	Host string `json:"host"`

	// Port Redis 端口
	// +kubebuilder:default=6379
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`

	// Type Redis 类型：node(单机)或 cluster(集群)
	// +kubebuilder:default="node"
	// +kubebuilder:validation:Enum=node;cluster
	Type string `json:"type,omitempty"`

	// Password Redis 密码(可选，如果 Redis 无密码可不填)
	// +optional
	Password string `json:"password,omitempty"`

	// TLS 是否启用 TLS 连接
	// +kubebuilder:default=false
	TLS bool `json:"tls,omitempty"`

	// NonBlock 是否使用非阻塞模式
	// +kubebuilder:default=true
	NonBlock bool `json:"nonBlock,omitempty"`

	// PingTimeout Ping 超时时间
	// +kubebuilder:default="30s"
	PingTimeout string `json:"pingTimeout,omitempty"`
}

// StorageConfig 对象存储配置
type StorageConfig struct {
	// Endpoint 存储端点地址(不含 http:// 或 https://)
	// 例如：minio.default.svc.cluster.local:9000
	// +kubebuilder:validation:Required
	Endpoint string `json:"endpoint"`

	// EndpointProxy 代理端点 提供给用户的访问入口
	// 例如：https://minio.default.svc.cluster.local:9000
	EndpointProxy string `json:"endpointProxy,omitempty"`

	// AccessKey 访问密钥 ID
	// +kubebuilder:validation:Required
	AccessKey string `json:"accessKey"`

	// SecretKey 访问密钥(明文，Operator 会自动存储到 Secret)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=1
	SecretKey string `json:"secretKey"`

	// Bucket 存储桶名称
	// +kubebuilder:validation:Required
	Bucket string `json:"bucket"`

	// TLS TLS 配置(如果 MinIO 启用了 HTTPS)
	// +optional
	TLS *MinIOTLSConfig `json:"tls,omitempty"`
}

// MinIOTLSConfig MinIO TLS 配置
type MinIOTLSConfig struct {
	// Enabled 是否启用 TLS
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// SecretName 包含 TLS 证书的 Secret 名称
	// Secret 必须包含以下键：
	// - public.crt: 公钥证书
	// - private.key: 私钥
	// 用户需要提前创建此 Secret
	// 当 Enabled=true 时此字段必填
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// TelemetryConfig 链路追踪配置
type TelemetryConfig struct {
	// Enabled 是否启用链路追踪
	// 如果为 false，后续配置将被忽略
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// JaegerEndpoint Jaeger Collector 端点地址
	// 例如：http://jaeger-collector.default.svc.cluster.local:14268/api/traces
	// 当 Enabled=true 时此字段必填
	// +optional
	JaegerEndpoint string `json:"jaegerEndpoint,omitempty"`

	// Sampler 采样率(0.0-1.0)
	// 1.0 表示采样所有请求，0.1 表示采样 10% 的请求
	// +kubebuilder:default="1.0"
	// +optional
	Sampler string `json:"sampler,omitempty"`

	// Batcher 批处理器类型
	// +kubebuilder:default="jaeger"
	// +optional
	Batcher string `json:"batcher,omitempty"`
}

// ServicesConfig 后端服务全局配置
type ServicesConfig struct {
	// GlobalTimeout 全局超时时间(毫秒)
	// +kubebuilder:default=30000
	GlobalTimeout int64 `json:"globalTimeout,omitempty"`

	// JWT JWT 配置
	// +kubebuilder:validation:Required
	JWT JWTConfig `json:"jwt"`

	// Portal Portal 配置
	// +optional
	Portal *PortalConfig `json:"portal,omitempty"`

	// WebhookToken Alertmanager Webhook Token(可选)
	// +optional
	WebhookToken string `json:"webhookToken,omitempty"`

	// InjectImage 注入容器使用的工具镜像
	// +kubebuilder:default="registry.cn-hangzhou.aliyuncs.com/kube-nova/network-multitool:latest"
	InjectImage string `json:"injectImage,omitempty"`

	// LeaderElection 全局 Leader Election 配置(可选)
	// 如果服务级别未配置，则使用此全局配置
	// +optional
	LeaderElection *LeaderElectionConfig `json:"leaderElection,omitempty"`

	// PortalAPI Portal API 服务配置
	// +optional
	PortalAPI *ServiceConfig `json:"portalAPI,omitempty"`

	// PortalRPC Portal RPC 服务配置
	// +optional
	PortalRPC *ServiceConfig `json:"portalRPC,omitempty"`

	// ManagerAPI Manager API 服务配置
	// +optional
	ManagerAPI *ServiceConfig `json:"managerAPI,omitempty"`

	// ManagerRPC Manager RPC 服务配置
	// +optional
	ManagerRPC *ServiceConfig `json:"managerRPC,omitempty"`

	// WorkloadAPI Workload API 服务配置
	// +optional
	WorkloadAPI *ServiceConfig `json:"workloadAPI,omitempty"`

	// ConsoleAPI Console API 服务配置
	// +optional
	ConsoleAPI *ServiceConfig `json:"consoleAPI,omitempty"`

	// ConsoleRPC Console RPC 服务配置
	// +optional
	ConsoleRPC *ServiceConfig `json:"consoleRPC,omitempty"`

	// DevopsAPI Devops API 服务配置
	// +optional
	DevopsAPI *ServiceConfig `json:"devopsAPI,omitempty"`

	// DevopsManagerRPC Devops Manager RPC 服务配置
	// +optional
	DevopsManagerRPC *ServiceConfig `json:"devopsManagerRPC,omitempty"`

	// DevopsPipelineRPC Devops Pipeline RPC 服务配置
	// +optional
	DevopsPipelineRPC *ServiceConfig `json:"devopsPipelineRPC,omitempty"`

	// DevopsQualityRPC Devops Quality RPC 服务配置(暂不部署)
	// +optional
	DevopsQualityRPC *ServiceConfig `json:"devopsQualityRPC,omitempty"`
}

// JWTConfig JWT 认证配置
type JWTConfig struct {
	// AccessSecret 访问令牌密钥(至少 32 字符)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=32
	AccessSecret string `json:"accessSecret"`

	// AccessExpire 访问令牌过期时间(秒)
	// +kubebuilder:default=86400
	AccessExpire int64 `json:"accessExpire,omitempty"`

	// RefreshSecret 刷新令牌密钥(至少 32 字符)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinLength=32
	RefreshSecret string `json:"refreshSecret"`

	// RefreshExpire 刷新令牌过期时间(秒)
	// +kubebuilder:default=604800
	RefreshExpire int64 `json:"refreshExpire,omitempty"`

	// RefreshAfter 刷新令牌生效时间(秒)
	// +kubebuilder:default=604800
	RefreshAfter int64 `json:"refreshAfter,omitempty"`
}

// LeaderElectionConfig Leader Election 配置
type LeaderElectionConfig struct {
	// Enabled 是否启用 Leader Election 模式
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// LeaseName Lease 资源名称（会自动创建）
	// 如果不指定，将使用服务名称作为默认值
	// 注意：Lease 资源会自动创建在 KubeNova 资源所在的命名空间中
	// +optional
	LeaseName string `json:"leaseName,omitempty"`

	// LeaseDuration 租约有效期
	// +kubebuilder:default="15s"
	// +optional
	LeaseDuration string `json:"leaseDuration,omitempty"`

	// RenewDeadline 续约超时
	// +kubebuilder:default="10s"
	// +optional
	RenewDeadline string `json:"renewDeadline,omitempty"`

	// RetryPeriod 重试间隔
	// +kubebuilder:default="2s"
	// +optional
	RetryPeriod string `json:"retryPeriod,omitempty"`
}

// PortalConfig Portal 门户配置
type PortalConfig struct {
	// Name Portal 名称
	// +kubebuilder:default="Kube-Nova 云原生平台"
	Name string `json:"name,omitempty"`

	// URL Portal 访问地址
	// +optional
	URL string `json:"url,omitempty"`

	// DemoMode 是否启用演示模式
	// +kubebuilder:default=false
	DemoMode bool `json:"demoMode,omitempty"`
}

// ServiceConfig 单个服务配置
type ServiceConfig struct {
	// Enabled 是否启用此服务
	// +kubebuilder:default=true
	Enabled *bool `json:"enabled,omitempty"`

	// Replicas 副本数
	// +kubebuilder:default=2
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Replicas int32 `json:"replicas,omitempty"`

	// Image 完整镜像名称(如果需要覆盖全局配置)
	// 例如：registry.cn-hangzhou.aliyuncs.com/kube-nova/portal-api:v1.0.0
	// +optional
	Image string `json:"image,omitempty"`

	// Resources 资源配置
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// Env 额外的环境变量
	// +optional
	Env []corev1.EnvVar `json:"env,omitempty"`

	// LeaderElection 服务级别的 Leader Election 配置
	// 如果配置，将覆盖全局 Leader Election 配置
	// +optional
	LeaderElection *LeaderElectionConfig `json:"leaderElection,omitempty"`
}

// WebConfig Web 前端配置
type WebConfig struct {
	// Replicas 副本数
	// +kubebuilder:default=3
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=100
	Replicas int32 `json:"replicas,omitempty"`

	// Image 完整镜像名称(如果需要覆盖全局配置)
	// +optional
	Image string `json:"image,omitempty"`

	// Resources 资源配置
	// +optional
	Resources *corev1.ResourceRequirements `json:"resources,omitempty"`

	// ExposeType 暴露方式：ingress 或 nodeport
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=ingress;nodeport
	ExposeType string `json:"exposeType"`

	// Ingress Ingress 配置(当 ExposeType=ingress 时必填)
	// +optional
	Ingress *IngressConfig `json:"ingress,omitempty"`

	// NodePort NodePort 配置(当 ExposeType=nodeport 时可选，不配置则使用自动分配的端口)
	// +optional
	NodePort *NodePortConfig `json:"nodePort,omitempty"`

	// MinIOProxy MinIO 代理配置
	// +optional
	MinIOProxy *MinIOProxyConfig `json:"minioProxy,omitempty"`

	// CustomNginxConfigMap 自定义 Nginx 配置的 ConfigMap 名称
	// 如果指定，将使用此 ConfigMap 替代默认的 Nginx 配置
	// ConfigMap 必须包含 nginx.conf 和 default.conf 两个 key
	// +optional
	CustomNginxConfigMap string `json:"customNginxConfigMap,omitempty"`
}

// IngressConfig Ingress 配置
type IngressConfig struct {
	// ClassName Ingress 类名
	// +kubebuilder:default="nginx"
	ClassName string `json:"className,omitempty"`

	// Host 域名
	// +kubebuilder:validation:Required
	Host string `json:"host"`

	// TLS TLS 配置
	// +optional
	TLS *IngressTLSConfig `json:"tls,omitempty"`

	// Annotations 额外的注解
	// +optional
	Annotations map[string]string `json:"annotations,omitempty"`
}

// IngressTLSConfig Ingress TLS 配置
type IngressTLSConfig struct {
	// Enabled 是否启用 TLS
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// SecretName TLS 证书 Secret 名称
	// Secret 必须包含 tls.crt 和 tls.key
	// 用户需要提前创建此 Secret
	// 当 Enabled=true 时此字段必填
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// NodePortConfig NodePort 配置
type NodePortConfig struct {
	// HTTPPort HTTP NodePort 端口(可选，不指定则自动分配)
	// +optional
	// +kubebuilder:validation:Minimum=30000
	// +kubebuilder:validation:Maximum=32767
	HTTPPort int32 `json:"httpPort,omitempty"`

	// HTTPS HTTPS 配置
	// +optional
	HTTPS *NodePortHTTPSConfig `json:"https,omitempty"`
}

// NodePortHTTPSConfig NodePort HTTPS 配置
type NodePortHTTPSConfig struct {
	// Enabled 是否启用 HTTPS
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// Port HTTPS NodePort 端口(可选，不指定则自动分配)
	// +optional
	// +kubebuilder:validation:Minimum=30000
	// +kubebuilder:validation:Maximum=32767
	Port int32 `json:"port,omitempty"`

	// SecretName TLS 证书 Secret 名称
	// Secret 必须包含 tls.crt 和 tls.key
	// 用户需要提前创建此 Secret
	// 当 Enabled=true 时此字段必填
	// +optional
	SecretName string `json:"secretName,omitempty"`
}

// MinIOProxyConfig MinIO 代理配置
type MinIOProxyConfig struct {
	// Enabled 是否在 Nginx 中启用 MinIO 代理
	// 启用后可以通过 Web 前端访问 MinIO 的对象
	// +kubebuilder:default=false
	Enabled bool `json:"enabled,omitempty"`

	// PathPrefix 代理路径前缀
	// 例如：/storage，则通过 http://web-host/storage/bucket/object 访问
	// +kubebuilder:default="/storage"
	PathPrefix string `json:"pathPrefix,omitempty"`

	// ProxyEndpoint 代理地址(可选，通常自动推断)
	// 如果手动配置，后端服务将通过此地址访问 MinIO
	// 例如：http://www.example.com/storage
	// 如果不配置，将根据 exposeType 自动推断：
	// - Ingress 模式：http(s)://域名/storage
	// - NodePort 模式：http://<NODE_IP>:<NODE_PORT>/storage
	// +optional
	ProxyEndpoint string `json:"proxyEndpoint,omitempty"`
}

// ========================================
// KubeNovaStatus - 状态定义
// ========================================

// KubeNovaStatus 定义了 KubeNova 的观测状态
type KubeNovaStatus struct {
	// Phase 当前部署阶段
	// +optional
	Phase DeploymentPhase `json:"phase,omitempty"`

	// Conditions 条件列表
	// +optional
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ComponentStatus 组件状态
	// +optional
	ComponentStatus ComponentStatusMap `json:"componentStatus,omitempty"`

	// AccessInfo 访问信息
	// +optional
	AccessInfo *AccessInfo `json:"accessInfo,omitempty"`

	// Message 状态消息
	// +optional
	Message string `json:"message,omitempty"`

	// LastUpdateTime 最后更新时间
	// +optional
	LastUpdateTime metav1.Time `json:"lastUpdateTime,omitempty"`

	// ObservedGeneration 观测到的 generation
	// +optional
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

// DeploymentPhase 部署阶段
type DeploymentPhase string

const (
	// PhaseValidating 正在验证配置
	PhaseValidating DeploymentPhase = "验证中"
	// PhasePending 等待部署
	PhasePending DeploymentPhase = "等待中"
	// PhaseCreating 正在创建资源
	PhaseCreating DeploymentPhase = "创建中"
	// PhaseReady 部署就绪
	PhaseReady DeploymentPhase = "就绪"
	// PhaseUpdating 正在更新
	PhaseUpdating DeploymentPhase = "更新中"
	// PhaseFailed 部署失败
	PhaseFailed DeploymentPhase = "失败"
	// PhaseDeleting 正在删除
	PhaseDeleting DeploymentPhase = "删除中"
)

// ComponentStatusMap 组件状态映射
type ComponentStatusMap struct {
	// Database 数据库状态
	// +optional
	Database *ComponentStatus `json:"database,omitempty"`

	// Cache 缓存状态
	// +optional
	Cache *ComponentStatus `json:"cache,omitempty"`

	// Storage 存储状态
	// +optional
	Storage *ComponentStatus `json:"storage,omitempty"`

	// Telemetry 链路追踪状态
	// +optional
	Telemetry *ComponentStatus `json:"telemetry,omitempty"`

	// Services 后端服务状态
	// +optional
	Services map[string]*ComponentStatus `json:"services,omitempty"`

	// Web Web 前端状态
	// +optional
	Web *ComponentStatus `json:"web,omitempty"`
}

// ComponentStatus 组件状态
type ComponentStatus struct {
	// State 状态
	// +optional
	State ComponentState `json:"state,omitempty"`

	// ReadyReplicas 就绪副本数
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// DesiredReplicas 期望副本数
	// +optional
	DesiredReplicas int32 `json:"desiredReplicas,omitempty"`

	// Message 状态消息
	// +optional
	Message string `json:"message,omitempty"`

	// LastTransitionTime 最后状态转换时间
	// +optional
	LastTransitionTime metav1.Time `json:"lastTransitionTime,omitempty"`
}

// ComponentState 组件状态
type ComponentState string

const (
	// ComponentStatePending 等待中
	ComponentStatePending ComponentState = "等待中"
	// ComponentStateCreating 创建中
	ComponentStateCreating ComponentState = "创建中"
	// ComponentStateRunning 运行中
	ComponentStateRunning ComponentState = "运行中"
	// ComponentStateReady 就绪
	ComponentStateReady ComponentState = "就绪"
	// ComponentStateFailed 失败
	ComponentStateFailed ComponentState = "失败"
	// ComponentStateUpdating 更新中
	ComponentStateUpdating ComponentState = "更新中"
)

// AccessInfo 访问信息
type AccessInfo struct {
	// WebURL Web 访问地址
	// +optional
	WebURL string `json:"webURL,omitempty"`

	// DatabaseEndpoint 数据库端点
	// +optional
	DatabaseEndpoint string `json:"databaseEndpoint,omitempty"`

	// CacheEndpoint 缓存端点
	// +optional
	CacheEndpoint string `json:"cacheEndpoint,omitempty"`

	// StorageEndpoint 存储端点
	// +optional
	StorageEndpoint string `json:"storageEndpoint,omitempty"`

	// StorageConsoleURL MinIO 控制台地址
	// +optional
	StorageConsoleURL string `json:"storageConsoleURL,omitempty"`

	// JaegerUIURL Jaeger UI 访问地址
	// +optional
	JaegerUIURL string `json:"jaegerUIURL,omitempty"`

	// ServiceEndpoints 服务端点映射
	// +optional
	ServiceEndpoints map[string]string `json:"serviceEndpoints,omitempty"`
}

// ========================================
// Condition Types
// ========================================

const (
	// ConditionTypeReady KubeNova 是否就绪
	ConditionTypeReady = "Ready"
	// ConditionTypeValidated 配置是否已验证
	ConditionTypeValidated = "Validated"
	// ConditionTypeDatabaseConnected 数据库是否连接成功
	ConditionTypeDatabaseConnected = "DatabaseConnected"
	// ConditionTypeCacheConnected 缓存是否连接成功
	ConditionTypeCacheConnected = "CacheConnected"
	// ConditionTypeStorageConnected 存储是否连接成功
	ConditionTypeStorageConnected = "StorageConnected"
	// ConditionTypeTelemetryReady 链路追踪是否就绪
	ConditionTypeTelemetryReady = "TelemetryReady"
	// ConditionTypeServicesReady 后端服务是否就绪
	ConditionTypeServicesReady = "ServicesReady"
	// ConditionTypeWebReady Web 前端是否就绪
	ConditionTypeWebReady = "WebReady"
)

// ========================================
// Kubebuilder Markers
// ========================================

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:scope=Namespaced,shortName=kn
//+kubebuilder:printcolumn:name="阶段",type=string,JSONPath=`.status.phase`
//+kubebuilder:printcolumn:name="Web",type=string,JSONPath=`.status.componentStatus.web.state`
//+kubebuilder:printcolumn:name="访问地址",type=string,JSONPath=`.status.accessInfo.webURL`
//+kubebuilder:printcolumn:name="运行时长",type="date",JSONPath=".metadata.creationTimestamp"

// KubeNova is the Schema for the KubeNova API
type KubeNova struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KubeNovaSpec   `json:"spec,omitempty"`
	Status KubeNovaStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// KubeNovaList contains a list of KubeNova
type KubeNovaList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KubeNova `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KubeNova{}, &KubeNovaList{})
}

// ========================================
// Deployment Mode Helpers
// ========================================

// IsK8sMode 检查是否为 K8s 管理模式(包含 all)
func (k *KubeNova) IsK8sMode() bool {
	return k.Spec.DeploymentMode == "" || k.Spec.DeploymentMode == "all" || k.Spec.DeploymentMode == "k8s"
}

// IsDevopsMode 检查是否为 DevOps 模式(包含 all)
func (k *KubeNova) IsDevopsMode() bool {
	return k.Spec.DeploymentMode == "" || k.Spec.DeploymentMode == "all" || k.Spec.DeploymentMode == "devops"
}

// IsAllMode 检查是否为全部部署模式
func (k *KubeNova) IsAllMode() bool {
	return k.Spec.DeploymentMode == "" || k.Spec.DeploymentMode == "all"
}

// GetDeploymentMode 获取部署模式，默认返回 all
func (k *KubeNova) GetDeploymentMode() DeploymentMode {
	if k.Spec.DeploymentMode == "" {
		return "all"
	}
	return k.Spec.DeploymentMode
}

// ========================================
// Helper Methods
// ========================================

// GetImageRegistry 获取镜像仓库配置
func (k *KubeNova) GetImageRegistry() ImageRegistryConfig {
	if k.Spec.ImageRegistry != nil {
		return *k.Spec.ImageRegistry
	}
	return ImageRegistryConfig{
		Registry:     "registry.cn-hangzhou.aliyuncs.com",
		Organization: "kube-nova",
		Tag:          "latest",
		PullPolicy:   corev1.PullAlways,
	}
}

// IsServiceEnabled 检查服务是否启用
func (s *ServiceConfig) IsServiceEnabled() bool {
	if s == nil {
		return true
	}
	if s.Enabled == nil {
		return true
	}
	return *s.Enabled
}

// GetReplicas 获取副本数
func (s *ServiceConfig) GetReplicas() int32 {
	if s == nil || s.Replicas <= 0 {
		return 2
	}
	return s.Replicas
}

// GetResources 获取资源配置
func (s *ServiceConfig) GetResources() *corev1.ResourceRequirements {
	if s == nil || s.Resources == nil {
		return nil
	}
	return s.Resources
}

// IsTelemetryEnabled 检查链路追踪是否启用
func (k *KubeNova) IsTelemetryEnabled() bool {
	return k.Spec.Telemetry != nil && k.Spec.Telemetry.Enabled
}

// ValidateTelemetryConfig 验证链路追踪配置
func (t *TelemetryConfig) ValidateTelemetryConfig() error {
	if !t.Enabled {
		return nil
	}
	if t.JaegerEndpoint == "" {
		return fmt.Errorf("链路追踪已启用但未配置 Jaeger 端点地址")
	}
	return nil
}

// ValidateMinIOTLS 验证 MinIO TLS 配置
func (tls *MinIOTLSConfig) ValidateMinIOTLS() error {
	if !tls.Enabled {
		return nil
	}
	if tls.SecretName == "" {
		return fmt.Errorf("MinIO TLS 已启用但未指定证书 Secret 名称")
	}
	return nil
}

// ValidateWebConfig 验证 Web 配置
func (w *WebConfig) ValidateWebConfig() error {
	switch w.ExposeType {
	case "ingress":
		if w.Ingress == nil {
			return fmt.Errorf("暴露方式为 ingress 但未配置 Ingress")
		}
		if w.Ingress.Host == "" {
			return fmt.Errorf("ingress 配置缺少域名")
		}
		if w.Ingress.TLS != nil && w.Ingress.TLS.Enabled && w.Ingress.TLS.SecretName == "" {
			return fmt.Errorf("ingress TLS 已启用但未指定证书 Secret 名称")
		}
	case "nodeport":
		// NodePort 可以为 nil，这种情况下使用自动分配的端口
		// 只有当 HTTPS 启用时才需要验证证书配置
		if w.NodePort != nil && w.NodePort.HTTPS != nil && w.NodePort.HTTPS.Enabled && w.NodePort.HTTPS.SecretName == "" {
			return fmt.Errorf("NodePort HTTPS 已启用但未指定证书 Secret 名称")
		}
	default:
		return fmt.Errorf("不支持的暴露方式: %s", w.ExposeType)
	}
	return nil
}

// GetCachePassword 获取 Redis 密码
func (c *CacheConfig) GetCachePassword() string {
	return c.Password
}

// GetDatabaseConnectionString 获取数据库连接字符串(用于内部)
func (d *DatabaseConfig) GetDatabaseConnectionString() string {
	return fmt.Sprintf("%s:%s@tcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local&timeout=10s",
		d.User, d.Password, d.Host, d.Port, d.Database)
}

// GetStorageEndpoint 获取存储端点(带协议)
func (s *StorageConfig) GetStorageEndpoint() string {
	if s.TLS != nil && s.TLS.Enabled {
		return "https://" + s.Endpoint
	}
	return "http://" + s.Endpoint
}

// GetCacheEndpoint 获取缓存端点
func (c *CacheConfig) GetCacheEndpoint() string {
	return fmt.Sprintf("%s:%d", c.Host, c.Port)
}

// GetDatabaseEndpoint 获取数据库端点
func (d *DatabaseConfig) GetDatabaseEndpoint() string {
	return fmt.Sprintf("%s:%d", d.Host, d.Port)
}

// GetWebReplicas 获取 Web 副本数
func (w *WebConfig) GetWebReplicas() int32 {
	if w.Replicas <= 0 {
		return 3
	}
	return w.Replicas
}

// IsMinIOProxyEnabled 检查 MinIO 代理是否启用
func (k *KubeNova) IsMinIOProxyEnabled() bool {
	return k.Spec.Web.MinIOProxy != nil && k.Spec.Web.MinIOProxy.Enabled
}

// GetMinIOProxyPath 获取 MinIO 代理路径
func (k *KubeNova) GetMinIOProxyPath() string {
	if k.Spec.Web.MinIOProxy == nil || k.Spec.Web.MinIOProxy.PathPrefix == "" {
		return "/storage"
	}
	return k.Spec.Web.MinIOProxy.PathPrefix
}

// GetMinIOProxyEndpoint 获取用户配置的 MinIO 代理端点
func (k *KubeNova) GetMinIOProxyEndpoint() string {
	if k.Spec.Web.MinIOProxy != nil && k.Spec.Web.MinIOProxy.ProxyEndpoint != "" {
		return k.Spec.Web.MinIOProxy.ProxyEndpoint
	}
	return ""
}

// GetMinIOEndpointForBackend 获取后端服务使用的 MinIO 端点
// - 未启用代理：minio-service:9000
// - Ingress 模式：www.example.com/storage
// - NodePort 模式：<NODE_IP>:<NODE_PORT>/storage
func (k *KubeNova) GetMinIOEndpointForBackend() string {
	// 如果未启用代理，直接返回实际 MinIO 地址
	if !k.IsMinIOProxyEnabled() {
		return k.Spec.Storage.Endpoint
	}

	// 如果用户手动配置了代理端点，使用用户配置
	if k.Spec.Web.MinIOProxy.ProxyEndpoint != "" {
		return k.Spec.Web.MinIOProxy.ProxyEndpoint
	}

	// 自动推断代理端点
	pathPrefix := k.GetMinIOProxyPath()

	if k.Spec.Web.ExposeType == "ingress" && k.Spec.Web.Ingress != nil {
		// Ingress 模式：返回域名 + pathPrefix
		// 例如：www.example.com/storage
		host := k.Spec.Web.Ingress.Host
		// 移除前导斜杠
		if len(pathPrefix) > 0 && pathPrefix[0] == '/' {
			pathPrefix = pathPrefix[1:]
		}
		return fmt.Sprintf("%s/%s", host, pathPrefix)
	}

	// NodePort 模式：返回占位符，实际地址在 Status 中更新
	// 例如：<NODE_IP>:<NODE_PORT>/storage
	// 移除前导斜杠
	if len(pathPrefix) > 0 && pathPrefix[0] == '/' {
		pathPrefix = pathPrefix[1:]
	}
	return fmt.Sprintf("<NODE_IP>:<NODE_PORT>/%s", pathPrefix)
}

// GetMinIOEndpointForBackendWithNodeInfo 获取后端服务使用的 MinIO 端点(带 Node 信息)
// 用于 controller 在运行时生成实际地址
func (k *KubeNova) GetMinIOEndpointForBackendWithNodeInfo(nodeIP string, nodePort int32) string {
	// 如果未启用代理，直接返回实际 MinIO 地址
	if !k.IsMinIOProxyEnabled() {
		return k.Spec.Storage.Endpoint
	}

	// 如果用户手动配置了代理端点，使用用户配置
	if k.Spec.Web.MinIOProxy.ProxyEndpoint != "" {
		return k.Spec.Web.MinIOProxy.ProxyEndpoint
	}

	// 自动推断代理端点
	pathPrefix := k.GetMinIOProxyPath()

	if k.Spec.Web.ExposeType == "ingress" && k.Spec.Web.Ingress != nil {
		protocol := "http"
		if k.Spec.Web.Ingress.TLS != nil && k.Spec.Web.Ingress.TLS.Enabled {
			protocol = "https"
		}
		host := k.Spec.Web.Ingress.Host
		if len(pathPrefix) > 0 && pathPrefix[0] == '/' {
			pathPrefix = pathPrefix[1:]
		}
		return fmt.Sprintf("%s://%s/%s", protocol, host, pathPrefix)
	}

	// NodePort 模式：使用实际的 Node IP 和 NodePort，并判断 HTTPS
	protocol := "http"
	if k.Spec.Web.NodePort != nil && k.Spec.Web.NodePort.HTTPS != nil && k.Spec.Web.NodePort.HTTPS.Enabled {
		protocol = "https"
	}
	if nodeIP == "" {
		nodeIP = "<NODE_IP>"
	}
	if nodePort == 0 {
		if protocol == "https" {
			nodePort = 30443 // HTTPS 默认值
		} else {
			nodePort = 30080 // HTTP 默认值
		}
	}
	if len(pathPrefix) > 0 && pathPrefix[0] == '/' {
		pathPrefix = pathPrefix[1:]
	}
	return fmt.Sprintf("%s://%s:%d/%s", protocol, nodeIP, nodePort, pathPrefix)
}

// GetMaxOpenConns 获取最大打开连接数
func (d *DatabaseConfig) GetMaxOpenConns() int32 {
	if d.MaxOpenConns <= 0 {
		return 100
	}
	return d.MaxOpenConns
}

// GetMaxIdleConns 获取最大空闲连接数
func (d *DatabaseConfig) GetMaxIdleConns() int32 {
	if d.MaxIdleConns <= 0 {
		return 50
	}
	return d.MaxIdleConns
}

// GetConnMaxLifetime 获取连接最大生命周期
func (d *DatabaseConfig) GetConnMaxLifetime() string {
	if d.ConnMaxLifetime == "" {
		return "30m"
	}
	return d.ConnMaxLifetime
}

// GetEffectiveLeaderElection 获取服务的有效 Leader Election 配置
// 优先级：服务级别 > 全局级别
func (s *ServiceConfig) GetEffectiveLeaderElection(globalConfig *LeaderElectionConfig) *LeaderElectionConfig {
	// 如果服务级别配置了，使用服务级别配置
	if s != nil && s.LeaderElection != nil {
		return s.LeaderElection
	}
	// 否则使用全局配置
	return globalConfig
}

// IsLeaderElectionEnabled 检查 Leader Election 是否启用
func (l *LeaderElectionConfig) IsLeaderElectionEnabled() bool {
	return l != nil && l.Enabled
}

// GetLeaseName 获取 Lease 名称，如果未配置则返回默认值
func (l *LeaderElectionConfig) GetLeaseName(defaultName string) string {
	if l == nil || l.LeaseName == "" {
		return defaultName
	}
	return l.LeaseName
}

// GetLeaseDuration 获取租约有效期
func (l *LeaderElectionConfig) GetLeaseDuration() string {
	if l == nil || l.LeaseDuration == "" {
		return "15s"
	}
	return l.LeaseDuration
}

// GetRenewDeadline 获取续约超时
func (l *LeaderElectionConfig) GetRenewDeadline() string {
	if l == nil || l.RenewDeadline == "" {
		return "10s"
	}
	return l.RenewDeadline
}

// GetRetryPeriod 获取重试间隔
func (l *LeaderElectionConfig) GetRetryPeriod() string {
	if l == nil || l.RetryPeriod == "" {
		return "2s"
	}
	return l.RetryPeriod
}
