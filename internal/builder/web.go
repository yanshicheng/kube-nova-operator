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
	networkingv1 "k8s.io/api/networking/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/intstr"

	kubenovav1 "github.com/yanshicheng/kube-nova-operator/api/v1"
)

// WebResources Web 资源
type WebResources struct {
	Deployment     *appsv1.Deployment
	Service        *corev1.Service
	Ingress        *networkingv1.Ingress
	NginxConfigMap *corev1.ConfigMap
}

// BuildWebResources 构建 Web 资源
func BuildWebResources(kn *kubenovav1.KubeNova, namespace string) *WebResources {
	resources := &WebResources{}

	// 构建 Nginx ConfigMap（如果用户没有自定义）
	if kn.Spec.Web.CustomNginxConfigMap == "" {
		resources.NginxConfigMap = buildNginxConfigMap(kn, namespace)
	}

	// 构建 Deployment
	resources.Deployment = buildWebDeployment(kn, namespace)

	// 构建 Service
	resources.Service = buildWebService(kn, namespace)

	// 构建 Ingress（如果使用 Ingress 模式）
	if kn.Spec.Web.ExposeType == "ingress" {
		resources.Ingress = buildWebIngress(kn, namespace)
	}

	return resources
}

// buildWebDeployment 构建 Web Deployment
func buildWebDeployment(kn *kubenovav1.KubeNova, namespace string) *appsv1.Deployment {
	registry := kn.GetImageRegistry()
	replicas := kn.Spec.Web.GetWebReplicas()

	// 构建镜像名称
	image := fmt.Sprintf("%s/%s/kube-nova-web:%s", registry.Registry, registry.Organization, registry.Tag)
	if kn.Spec.Web.Image != "" {
		image = kn.Spec.Web.Image
	}

	labels := map[string]string{
		"app":                          "kube-nova-web",
		"tier":                         "frontend",
		"app.kubernetes.io/name":       "kube-nova",
		"app.kubernetes.io/instance":   kn.Name,
		"app.kubernetes.io/component":  "web",
		"app.kubernetes.io/managed-by": "kube-nova-operator",
	}

	deployment := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-nova-web",
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
					"app": "kube-nova-web",
				},
			},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: labels,
				},
				Spec: corev1.PodSpec{
					Affinity: &corev1.Affinity{
						PodAntiAffinity: &corev1.PodAntiAffinity{
							PreferredDuringSchedulingIgnoredDuringExecution: []corev1.WeightedPodAffinityTerm{
								{
									Weight: 100,
									PodAffinityTerm: corev1.PodAffinityTerm{
										LabelSelector: &metav1.LabelSelector{
											MatchExpressions: []metav1.LabelSelectorRequirement{
												{
													Key:      "app",
													Operator: metav1.LabelSelectorOpIn,
													Values:   []string{"kube-nova-web"},
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
							Name:            "kube-nova-web",
							Image:           image,
							ImagePullPolicy: registry.PullPolicy,
							Ports:           getWebPorts(kn),
							Env: []corev1.EnvVar{
								{
									Name:  "TZ",
									Value: "Asia/Shanghai",
								},
								{
									Name:  "NGINX_WORKER_PROCESSES",
									Value: "auto",
								},
								{
									Name:  "NGINX_WORKER_CONNECTIONS",
									Value: "4096",
								},
							},
							VolumeMounts: getWebVolumeMounts(kn),
							LivenessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/health",
										Port:   intstr.FromInt(80),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 10,
								PeriodSeconds:       10,
								TimeoutSeconds:      3,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
							ReadinessProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/health",
										Port:   intstr.FromInt(80),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 5,
								PeriodSeconds:       5,
								TimeoutSeconds:      3,
								SuccessThreshold:    1,
								FailureThreshold:    3,
							},
							StartupProbe: &corev1.Probe{
								ProbeHandler: corev1.ProbeHandler{
									HTTPGet: &corev1.HTTPGetAction{
										Path:   "/health",
										Port:   intstr.FromInt(80),
										Scheme: corev1.URISchemeHTTP,
									},
								},
								InitialDelaySeconds: 0,
								PeriodSeconds:       2,
								TimeoutSeconds:      3,
								SuccessThreshold:    1,
								FailureThreshold:    30,
							},
							Resources: getWebResourceRequirements(kn),
						},
					},
					Volumes:                       getWebVolumes(kn, namespace),
					DNSPolicy:                     corev1.DNSClusterFirst,
					RestartPolicy:                 corev1.RestartPolicyAlways,
					TerminationGracePeriodSeconds: int64Ptr(30),
				},
			},
		},
	}

	// 添加镜像拉取密钥
	if len(registry.PullSecrets) > 0 {
		imagePullSecrets := make([]corev1.LocalObjectReference, 0, len(registry.PullSecrets))
		for _, secret := range registry.PullSecrets {
			imagePullSecrets = append(imagePullSecrets, corev1.LocalObjectReference{Name: secret})
		}
		deployment.Spec.Template.Spec.ImagePullSecrets = imagePullSecrets
	}

	return deployment
}

// getWebPorts 获取 Web 容器端口
func getWebPorts(kn *kubenovav1.KubeNova) []corev1.ContainerPort {
	ports := []corev1.ContainerPort{
		{
			Name:          "http",
			ContainerPort: 80,
			Protocol:      corev1.ProtocolTCP,
		},
	}

	// 如果启用 HTTPS (NodePort 模式)
	if kn.Spec.Web.ExposeType == "nodeport" &&
		kn.Spec.Web.NodePort != nil &&
		kn.Spec.Web.NodePort.HTTPS != nil &&
		kn.Spec.Web.NodePort.HTTPS.Enabled {
		ports = append(ports, corev1.ContainerPort{
			Name:          "https",
			ContainerPort: 443,
			Protocol:      corev1.ProtocolTCP,
		})
	}

	return ports
}

// buildWebService 构建 Web Service
func buildWebService(kn *kubenovav1.KubeNova, namespace string) *corev1.Service {
	labels := map[string]string{
		"app":                          "kube-nova-web",
		"tier":                         "frontend",
		"app.kubernetes.io/name":       "kube-nova",
		"app.kubernetes.io/instance":   kn.Name,
		"app.kubernetes.io/component":  "web",
		"app.kubernetes.io/managed-by": "kube-nova-operator",
	}

	service := &corev1.Service{
		ObjectMeta: metav1.ObjectMeta{
			Name:      "kube-nova-web",
			Namespace: namespace,
			Labels:    labels,
		},
		Spec: corev1.ServiceSpec{
			Selector: map[string]string{
				"app": "kube-nova-web",
			},
			Ports: []corev1.ServicePort{
				{
					Name:       "http",
					Port:       80,
					TargetPort: intstr.FromInt(80),
					Protocol:   corev1.ProtocolTCP,
				},
			},
		},
	}

	// 根据暴露类型配置 Service
	if kn.Spec.Web.ExposeType == "ingress" {
		// Ingress 模式：使用 ClusterIP
		service.Spec.Type = corev1.ServiceTypeClusterIP
	} else if kn.Spec.Web.ExposeType == "nodeport" {
		// NodePort 模式
		service.Spec.Type = corev1.ServiceTypeNodePort
		service.Spec.SessionAffinity = corev1.ServiceAffinityClientIP
		service.Spec.SessionAffinityConfig = &corev1.SessionAffinityConfig{
			ClientIP: &corev1.ClientIPConfig{
				TimeoutSeconds: int32Ptr(10800), // 3 小时
			},
		}
		service.Spec.ExternalTrafficPolicy = corev1.ServiceExternalTrafficPolicyTypeCluster

		// 设置 HTTP NodePort
		if kn.Spec.Web.NodePort != nil && kn.Spec.Web.NodePort.HTTPPort > 0 {
			service.Spec.Ports[0].NodePort = kn.Spec.Web.NodePort.HTTPPort
		}

		// 如果启用 HTTPS，添加 HTTPS 端口
		if kn.Spec.Web.NodePort != nil &&
			kn.Spec.Web.NodePort.HTTPS != nil &&
			kn.Spec.Web.NodePort.HTTPS.Enabled {
			httpsPort := corev1.ServicePort{
				Name:       "https",
				Port:       443,
				TargetPort: intstr.FromInt(443),
				Protocol:   corev1.ProtocolTCP,
			}
			if kn.Spec.Web.NodePort.HTTPS.Port > 0 {
				httpsPort.NodePort = kn.Spec.Web.NodePort.HTTPS.Port
			}
			service.Spec.Ports = append(service.Spec.Ports, httpsPort)
		}
	}

	return service
}

// buildWebIngress 构建 Web Ingress
func buildWebIngress(kn *kubenovav1.KubeNova, namespace string) *networkingv1.Ingress {
	if kn.Spec.Web.Ingress == nil {
		return nil
	}

	labels := map[string]string{
		"app":                          "kube-nova-web",
		"tier":                         "frontend",
		"app.kubernetes.io/name":       "kube-nova",
		"app.kubernetes.io/instance":   kn.Name,
		"app.kubernetes.io/component":  "web",
		"app.kubernetes.io/managed-by": "kube-nova-operator",
	}

	pathTypePrefix := networkingv1.PathTypePrefix

	ingress := &networkingv1.Ingress{
		ObjectMeta: metav1.ObjectMeta{
			Name:        "kube-nova-web",
			Namespace:   namespace,
			Labels:      labels,
			Annotations: getIngressAnnotations(kn),
		},
		Spec: networkingv1.IngressSpec{
			IngressClassName: &kn.Spec.Web.Ingress.ClassName,
			Rules: []networkingv1.IngressRule{
				{
					Host: kn.Spec.Web.Ingress.Host,
					IngressRuleValue: networkingv1.IngressRuleValue{
						HTTP: &networkingv1.HTTPIngressRuleValue{
							Paths: []networkingv1.HTTPIngressPath{
								{
									Path:     "/",
									PathType: &pathTypePrefix,
									Backend: networkingv1.IngressBackend{
										Service: &networkingv1.IngressServiceBackend{
											Name: "kube-nova-web",
											Port: networkingv1.ServiceBackendPort{
												Number: 80,
											},
										},
									},
								},
							},
						},
					},
				},
			},
		},
	}

	// 配置 TLS
	if kn.Spec.Web.Ingress.TLS != nil && kn.Spec.Web.Ingress.TLS.Enabled {
		if kn.Spec.Web.Ingress.TLS.SecretName != "" {
			ingress.Spec.TLS = []networkingv1.IngressTLS{
				{
					Hosts:      []string{kn.Spec.Web.Ingress.Host},
					SecretName: kn.Spec.Web.Ingress.TLS.SecretName,
				},
			}
		}
	}

	return ingress
}

// getIngressAnnotations 获取 Ingress 注解
func getIngressAnnotations(kn *kubenovav1.KubeNova) map[string]string {
	annotations := map[string]string{
		"nginx.ingress.kubernetes.io/ssl-redirect":           "false",
		"nginx.ingress.kubernetes.io/force-ssl-redirect":     "false",
		"nginx.ingress.kubernetes.io/proxy-body-size":        "1024m",
		"nginx.ingress.kubernetes.io/proxy-connect-timeout":  "600",
		"nginx.ingress.kubernetes.io/proxy-send-timeout":     "600",
		"nginx.ingress.kubernetes.io/proxy-read-timeout":     "600",
		"nginx.ingress.kubernetes.io/proxy-buffer-size":      "8k",
		"nginx.ingress.kubernetes.io/proxy-buffers-number":   "4",
		"nginx.ingress.kubernetes.io/websocket-services":     "kube-nova-web",
		"nginx.ingress.kubernetes.io/proxy-http-version":     "1.1",
		"nginx.ingress.kubernetes.io/enable-cors":            "true",
		"nginx.ingress.kubernetes.io/cors-allow-methods":     "GET, POST, PUT, DELETE, OPTIONS, PATCH",
		"nginx.ingress.kubernetes.io/cors-allow-origin":      "*",
		"nginx.ingress.kubernetes.io/cors-allow-credentials": "true",
		"nginx.ingress.kubernetes.io/cors-max-age":           "3600",
		"nginx.ingress.kubernetes.io/limit-rps":              "100",
		"nginx.ingress.kubernetes.io/limit-connections":      "50",
		"nginx.ingress.kubernetes.io/affinity":               "cookie",
		"nginx.ingress.kubernetes.io/affinity-mode":          "persistent",
		"nginx.ingress.kubernetes.io/session-cookie-name":    "route",
		"nginx.ingress.kubernetes.io/session-cookie-hash":    "sha1",
	}

	// 如果启用了 TLS，设置 SSL 重定向
	if kn.Spec.Web.Ingress != nil &&
		kn.Spec.Web.Ingress.TLS != nil &&
		kn.Spec.Web.Ingress.TLS.Enabled {
		annotations["nginx.ingress.kubernetes.io/ssl-redirect"] = "true"
		annotations["nginx.ingress.kubernetes.io/force-ssl-redirect"] = "true"
	}

	// 合并用户自定义注解
	if kn.Spec.Web.Ingress != nil && len(kn.Spec.Web.Ingress.Annotations) > 0 {
		for k, v := range kn.Spec.Web.Ingress.Annotations {
			annotations[k] = v
		}
	}

	return annotations
}

// getWebResourceRequirements 获取 Web 资源配置
func getWebResourceRequirements(kn *kubenovav1.KubeNova) corev1.ResourceRequirements {
	if kn.Spec.Web.Resources != nil {
		return *kn.Spec.Web.Resources
	}

	return corev1.ResourceRequirements{
		Requests: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("128Mi"),
			corev1.ResourceCPU:    resource.MustParse("100m"),
		},
		Limits: corev1.ResourceList{
			corev1.ResourceMemory: resource.MustParse("512Mi"),
			corev1.ResourceCPU:    resource.MustParse("500m"),
		},
	}
}

// getWebVolumeMounts 获取 Web VolumeMounts
func getWebVolumeMounts(kn *kubenovav1.KubeNova) []corev1.VolumeMount {
	mounts := []corev1.VolumeMount{
		{
			Name:      "cache",
			MountPath: "/var/cache/nginx",
		},
		{
			Name:      "logs",
			MountPath: "/var/log/nginx",
		},
		{
			Name:      "run",
			MountPath: "/var/run",
		},
	}

	// 使用自定义 ConfigMap 或默认生成的 ConfigMap
	if kn.Spec.Web.CustomNginxConfigMap != "" {

	}

	mounts = append(mounts,
		corev1.VolumeMount{
			Name:      "nginx-config",
			MountPath: "/etc/nginx/nginx.conf",
			SubPath:   "nginx.conf",
		},
		corev1.VolumeMount{
			Name:      "nginx-config",
			MountPath: "/etc/nginx/conf.d/default.conf",
			SubPath:   "default.conf",
		},
	)

	// 如果启用 HTTPS (NodePort 模式)，挂载证书
	if kn.Spec.Web.ExposeType == "nodeport" &&
		kn.Spec.Web.NodePort != nil &&
		kn.Spec.Web.NodePort.HTTPS != nil &&
		kn.Spec.Web.NodePort.HTTPS.Enabled &&
		kn.Spec.Web.NodePort.HTTPS.SecretName != "" {
		mounts = append(mounts, corev1.VolumeMount{
			Name:      "tls-certs",
			MountPath: "/etc/nginx/certs",
			ReadOnly:  true,
		})
	}

	return mounts
}

// getWebVolumes 获取 Web Volumes
func getWebVolumes(kn *kubenovav1.KubeNova, namespace string) []corev1.Volume {
	configMapName := "frontend-nginx-config"
	if kn.Spec.Web.CustomNginxConfigMap != "" {
		configMapName = kn.Spec.Web.CustomNginxConfigMap
	}

	volumes := []corev1.Volume{
		{
			Name: "nginx-config",
			VolumeSource: corev1.VolumeSource{
				ConfigMap: &corev1.ConfigMapVolumeSource{
					LocalObjectReference: corev1.LocalObjectReference{
						Name: configMapName,
					},
					Items: []corev1.KeyToPath{
						{
							Key:  "nginx.conf",
							Path: "nginx.conf",
						},
						{
							Key:  "default.conf",
							Path: "default.conf",
						},
					},
				},
			},
		},
		{
			Name: "cache",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: resource.NewQuantity(1*1024*1024*1024, resource.BinarySI), // 1Gi
				},
			},
		},
		{
			Name: "logs",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: resource.NewQuantity(500*1024*1024, resource.BinarySI), // 500Mi
				},
			},
		},
		{
			Name: "run",
			VolumeSource: corev1.VolumeSource{
				EmptyDir: &corev1.EmptyDirVolumeSource{
					SizeLimit: resource.NewQuantity(10*1024*1024, resource.BinarySI), // 10Mi
				},
			},
		},
	}

	// 如果启用 HTTPS (NodePort 模式)，添加证书 Volume
	if kn.Spec.Web.ExposeType == "nodeport" &&
		kn.Spec.Web.NodePort != nil &&
		kn.Spec.Web.NodePort.HTTPS != nil &&
		kn.Spec.Web.NodePort.HTTPS.Enabled &&
		kn.Spec.Web.NodePort.HTTPS.SecretName != "" {
		volumes = append(volumes, corev1.Volume{
			Name: "tls-certs",
			VolumeSource: corev1.VolumeSource{
				Secret: &corev1.SecretVolumeSource{
					SecretName: kn.Spec.Web.NodePort.HTTPS.SecretName,
					Optional:   boolPtr(false),
				},
			},
		})
	}

	return volumes
}
