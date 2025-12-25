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

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	kubenovav1 "github.com/yanshicheng/kube-nova-operator/api/v1"
)

const (
	// ServiceAccountName 所有服务使用的 ServiceAccount 名称
	ServiceAccountName = "kube-nova-sa"
)

// ServiceResources 服务资源
type ServiceResources struct {
	Deployment *appsv1.Deployment
	Service    *corev1.Service
}

// BuildAllServices 构建所有后端服务
func BuildAllServices(kn *kubenovav1.KubeNova, namespace string) map[string]*ServiceResources {
	services := make(map[string]*ServiceResources)

	registry := kn.GetImageRegistry()

	// Portal API
	if kn.Spec.Services.PortalAPI == nil || kn.Spec.Services.PortalAPI.IsServiceEnabled() {
		services["portal-api"] = buildService(kn, namespace, &serviceConfig{
			Name:          "portal-api",
			Port:          8810,
			TargetPort:    8810,
			MetricsPort:   9999,
			ConfigMapName: "portal-api-config",
			Registry:      registry,
			ServiceConfig: kn.Spec.Services.PortalAPI,
			Component:     "api",
		})
	}

	// Portal RPC - 需要特殊处理 MinIO 证书
	if kn.Spec.Services.PortalRPC == nil || kn.Spec.Services.PortalRPC.IsServiceEnabled() {
		services["portal-rpc"] = buildService(kn, namespace, &serviceConfig{
			Name:           "portal-rpc",
			Port:           30010,
			TargetPort:     30010,
			MetricsPort:    9999,
			ConfigMapName:  "portal-rpc-config",
			Registry:       registry,
			ServiceConfig:  kn.Spec.Services.PortalRPC,
			Component:      "rpc",
			NeedMinIOCerts: kn.Spec.Storage.TLS != nil && kn.Spec.Storage.TLS.Enabled,
			MinIOCertSecret: func() string {
				if kn.Spec.Storage.TLS != nil && kn.Spec.Storage.TLS.Enabled {
					return kn.Spec.Storage.TLS.SecretName
				}
				return ""
			}(),
		})
	}

	// Manager API
	if kn.Spec.Services.ManagerAPI == nil || kn.Spec.Services.ManagerAPI.IsServiceEnabled() {
		services["manager-api"] = buildService(kn, namespace, &serviceConfig{
			Name:          "manager-api",
			Port:          8811,
			TargetPort:    8811,
			MetricsPort:   9999,
			ConfigMapName: "manager-api-config",
			Registry:      registry,
			ServiceConfig: kn.Spec.Services.ManagerAPI,
			Component:     "api",
		})
	}

	// Manager RPC
	if kn.Spec.Services.ManagerRPC == nil || kn.Spec.Services.ManagerRPC.IsServiceEnabled() {
		services["manager-rpc"] = buildService(kn, namespace, &serviceConfig{
			Name:          "manager-rpc",
			Port:          30011,
			TargetPort:    30011,
			MetricsPort:   9999,
			ConfigMapName: "manager-rpc-config",
			Registry:      registry,
			ServiceConfig: kn.Spec.Services.ManagerRPC,
			Component:     "rpc",
		})
	}

	// Workload API
	if kn.Spec.Services.WorkloadAPI == nil || kn.Spec.Services.WorkloadAPI.IsServiceEnabled() {
		services["workload-api"] = buildService(kn, namespace, &serviceConfig{
			Name:          "workload-api",
			Port:          8812,
			TargetPort:    8812,
			MetricsPort:   9999,
			ConfigMapName: "workload-api-config",
			Registry:      registry,
			ServiceConfig: kn.Spec.Services.WorkloadAPI,
			Component:     "api",
		})
	}

	// Console API
	if kn.Spec.Services.ConsoleAPI == nil || kn.Spec.Services.ConsoleAPI.IsServiceEnabled() {
		services["console-api"] = buildService(kn, namespace, &serviceConfig{
			Name:          "console-api",
			Port:          8818,
			TargetPort:    8818,
			MetricsPort:   9999,
			ConfigMapName: "console-api-config",
			Registry:      registry,
			ServiceConfig: kn.Spec.Services.ConsoleAPI,
			Component:     "api",
			NeedCache:     true,
		})
	}

	// Console RPC
	if kn.Spec.Services.ConsoleRPC == nil || kn.Spec.Services.ConsoleRPC.IsServiceEnabled() {
		services["console-rpc"] = buildService(kn, namespace, &serviceConfig{
			Name:          "console-rpc",
			Port:          30018,
			TargetPort:    30018,
			MetricsPort:   9999,
			ConfigMapName: "console-rpc-config",
			Registry:      registry,
			ServiceConfig: kn.Spec.Services.ConsoleRPC,
			Component:     "rpc",
			NeedCache:     true,
		})
	}

	return services
}

// serviceConfig 服务配置
type serviceConfig struct {
	Name            string
	Port            int32
	TargetPort      int32
	MetricsPort     int32
	ConfigMapName   string
	Registry        kubenovav1.ImageRegistryConfig
	ServiceConfig   *kubenovav1.ServiceConfig
	Component       string
	NeedMinIOCerts  bool
	MinIOCertSecret string
	NeedCache       bool
}

// buildService 构建单个服务
func buildService(kn *kubenovav1.KubeNova, namespace string, cfg *serviceConfig) *ServiceResources {
	return &ServiceResources{
		Deployment: buildDeployment(kn, namespace, cfg),
		Service:    buildK8sService(namespace, cfg),
	}
}

// buildDeployment 构建 Deployment
func buildDeployment(kn *kubenovav1.KubeNova, namespace string, cfg *serviceConfig) *appsv1.Deployment {
	replicas := cfg.ServiceConfig.GetReplicas()

	// 构建镜像名称
	image := fmt.Sprintf("%s/%s/%s:%s", cfg.Registry.Registry, cfg.Registry.Organization, cfg.Name, cfg.Registry.Tag)
	if cfg.ServiceConfig != nil && cfg.ServiceConfig.Image != "" {
		image = cfg.ServiceConfig.Image
	}

	labels := map[string]string{
		"app":                          cfg.Name,
		"component":                    cfg.Component,
		"app.kubernetes.io/name":       "kube-nova",
		"app.kubernetes.io/instance":   kn.Name,
		"app.kubernetes.io/component":  cfg.Component,
		"app.kubernetes.io/managed-by": "kube-nova-operator",
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Strategy: appsv1.DeploymentStrategy{
				Type: appsv1.RollingUpdateDeploymentStrategyType,
				RollingUpdate: &appsv1.RollingUpdateDeployment{
					MaxSurge:       &intstr.IntOrString{Type: intstr.Int, IntVal: 1},
					MaxUnavailable: &intstr.IntOrString{Type: intstr.Int, IntVal: 0},
				},
			},
			Selector: &metav1.LabelSelector{
				MatchLabels: map[string]string{
					"app": cfg.Name,
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
					Annotations: map[string]string{
						"prometheus.io/scrape": "true",
						"prometheus.io/port":   "9999",
						"prometheus.io/path":   "/metrics",
					},
				},
				Spec: corev1.PodSpec{
					ServiceAccountName: ServiceAccountName,
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 50,
									PodAffinityTerm: corev1.PodAffinityTerm{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      "app",
													Operator: metav1.LabelSelectorOpIn,
													Values:   []string{cfg.Name},
												},
											},
										},
										TopologyKey: "kubernetes.io/hostname",
									},
								},
							},
						},
					},
					Containers: []corev1.Container{
						{
							Name:            cfg.Name,
							Image:           image,
							ImagePullPolicy: cfg.Registry.PullPolicy,
							Ports: []corev1.ContainerPort{
								{
									ContainerPort: cfg.TargetPort,
									Protocol:      corev1.ProtocolTCP,
								},
								{
									Name:          "metrics",
									ContainerPort: cfg.MetricsPort,
									Protocol:      corev1.ProtocolTCP,
								},
							},
							EnvFrom: []corev1.EnvFromSource{
								{
									SecretRef: &corev1.SecretEnvSource{
										LocalObjectReference: corev1.LocalObjectReference{
											Name: "kube-nova-secret",
										},
									},
								},
							},
							Env: []corev1.EnvVar{
								{
									Name:  "TZ",
									Value: "Asia/Shanghai",
								},
								{
									Name: "POD_NAMESPACE",
									ValueFrom: &corev1.EnvVarSource{
										FieldRef: &corev1.ObjectFieldSelector{
											FieldPath: "metadata.namespace",
										},
									},
								},
							},
							StartupProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt32(cfg.MetricsPort),
									},
								},
								InitialDelaySeconds: 0,
								PeriodSeconds:       5,
								FailureThreshold:    12,
								TimeoutSeconds:      3,
							},
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt32(cfg.MetricsPort),
									},
								},
								InitialDelaySeconds: 30,
								PeriodSeconds:       10,
								FailureThreshold:    3,
								TimeoutSeconds:      3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path: "/healthz",
										Port: intstr.FromInt32(cfg.MetricsPort),
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       5,
								FailureThreshold:    3,
								TimeoutSeconds:      3,
							},
							Resources:       getServiceResources(cfg.ServiceConfig),
							VolumeMounts:    getServiceVolumeMounts(cfg),
							SecurityContext: getSecurityContext(),
						},
					},
					Volumes:                       getServiceVolumes(cfg),
					DNSPolicy:                     corev1.DNSClusterFirst,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					TerminationGracePeriodSeconds: int64Ptr(30),
				},
			},
		},
	}

	// 添加额外的环境变量
	if cfg.ServiceConfig != nil && len(cfg.ServiceConfig.Env) > 0 {
		deployment.Spec.Template.Spec.Containers[0].Env = append(
			deployment.Spec.Template.Spec.Containers[0].Env,
			cfg.ServiceConfig.Env...,
		)
	}

	// 添加镜像拉取密钥
	if len(cfg.Registry.PullSecrets) > 0 {
		imagePullSecrets := make([]corev1.LocalObjectReference, 0, len(cfg.Registry.PullSecrets))
		for _, secret := range cfg.Registry.PullSecrets {
			imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: secret})
		}
		deployment.Spec.Template.Spec.ImagePullSecrets = imagePullSecrets
	}

	return deployment
}

// buildK8sService 构建 Kubernetes Service
func buildK8sService(namespace string, cfg *serviceConfig) *corev1.Service {
	labels := map[string]string{
		"app":       cfg.Name,
		"component": cfg.Component,
	}

	return &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      cfg.Name,
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Type: corev1.ServiceTypeClusterIP,
			Selector: map[string]string{
				"app": cfg.Name,
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       cfg.Port,
					TargetPort: intstr.FromInt32(cfg.TargetPort),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}
}

// getServiceResources 获取服务资源配置
func getServiceResources(sc *kubenovav1.ServiceConfig) corev1.ResourceRequirements {
	if sc != nil && sc.Resources != nil {
		return *sc.Resources
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("256Mi"),
			corev1.ResourceCPU:    resource.MustParse("200m"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("512Mi"),
			corev1.ResourceCPU:    resource.MustParse("1000m"),
		},
	}
}

// getServiceVolumeMounts 获取服务 VolumeMounts
func getServiceVolumeMounts(cfg *serviceConfig) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      "config",
			MountPath: "/app/etc",
			ReadOnly:  true,
		},
	}

	// 如果需要 MinIO 证书（仅 portal-rpc）
	if cfg.NeedMinIOCerts && cfg.MinIOCertSecret != "" {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "minio-certs",
			MountPath: "/app/etc/minio-certs",
			ReadOnly:  true,
		})
	}

	// 如果需要缓存目录
	if cfg.NeedCache {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "cache",
			MountPath: "/app/cache",
		})
	}

	return mounts
}

// getServiceVolumes 获取服务 Volumes
func getServiceVolumes(cfg *serviceConfig) []corev1.Volume {
	volumes := []corev1.Volume{
		{
			Name: "config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: cfg.ConfigMapName,
					},
				},
			},
		},
	}

	// 如果需要 MinIO 证书（仅 portal-rpc）
	if cfg.NeedMinIOCerts && cfg.MinIOCertSecret != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "minio-certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: cfg.MinIOCertSecret,
					Optional:   boolPtr(false),
					Items: []corev1.KeyToPath{
						{
							Key:  "public.crt",
							Path: "public.crt",
						},
						{
							Key:  "private.key",
							Path: "private.key",
						},
					},
				},
			},
		})
	}

	// 如果需要缓存目录
	if cfg.NeedCache {
		volumes = append(volumes, corev1.Volume{
			Name: "cache",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: resource.NewQuantity(1*1024*1024*1024, resource.BinarySI), // 1Gi
				},
			},
		})
	}

	return volumes
}

// getSecurityContext 获取安全上下文
func getSecurityContext() *corev1.SecurityContext {
	return &corev1.SecurityContext{
		RunAsNonRoot:             boolPtr(false),
		ReadOnlyRootFilesystem:   boolPtr(false),
		AllowPrivilegeEscalation: boolPtr(false),
		Capabilities: &corev1.Capabilities{
			Drop: []corev1.Capability{"ALL"},
		},
	}
}
