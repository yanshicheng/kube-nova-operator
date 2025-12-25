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

package controller

import (
	"context"
	"fmt"
	"reflect"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
	"sigs.k8s.io/controller-runtime/pkg/log"

	kubenovav1 "github.com/yanshicheng/kube-nova-operator/api/v1"
	"github.com/yanshicheng/kube-nova-operator/internal/builder"
	"github.com/yanshicheng/kube-nova-operator/internal/validator"
)

const (
	kubenovaFinalizer = "kubenova.io/finalizer"
)

// KubeNovaReconciler reconciles a KubeNova object
type KubeNovaReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

//+kubebuilder:rbac:groups=apps.ikubeops.com,resources=kubenova,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=apps.ikubeops.com,resources=kubenova/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=apps.ikubeops.com,resources=kubenova/finalizers,verbs=update
//+kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=services,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=configmaps,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=serviceaccounts,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=networking.k8s.io,resources=ingresses,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=core,resources=namespaces,verbs=get;list;watch
//+kubebuilder:rbac:groups=core,resources=nodes,verbs=get;list;watch
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterrolebindings,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=rbac.authorization.k8s.io,resources=clusterroles,verbs=get;list;watch;create;update;patch;delete

// Reconcile 主协调逻辑
func (r *KubeNovaReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("开始协调 KubeNova 资源", "资源名称", req.Name, "命名空间", req.Namespace)

	// 获取 KubeNova 实例
	kubenova := &kubenovav1.KubeNova{}
	if err := r.Get(ctx, req.NamespacedName, kubenova); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("KubeNova 资源已删除")
			return ctrl.Result{}, nil
		}
		logger.Error(err, "无法获取 KubeNova 资源")
		return ctrl.Result{}, err
	}

	// 处理删除逻辑
	if !kubenova.DeletionTimestamp.IsZero() {
		return r.reconcileDelete(ctx, kubenova)
	}

	// 添加 Finalizer
	if !controllerutil.ContainsFinalizer(kubenova, kubenovaFinalizer) {
		logger.Info("添加 Finalizer")
		controllerutil.AddFinalizer(kubenova, kubenovaFinalizer)
		if err := r.Update(ctx, kubenova); err != nil {
			logger.Error(err, "添加 Finalizer 失败")
			return ctrl.Result{}, err
		}
		return ctrl.Result{Requeue: true}, nil
	}

	// 检查 ObservedGeneration，避免不必要的 reconcile
	// 这个检查帮助我们避免对已经处理过且没有变化的资源重复执行昂贵的操作
	if kubenova.Status.ObservedGeneration == kubenova.Generation &&
		kubenova.Status.Phase == kubenovav1.PhaseReady {
		logger.Info("资源已处理且无变化，跳过 reconcile")
		return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
	}

	// ========== 阶段 1: 验证配置 ==========
	// 注意：reconcileValidation 函数内部只修改内存中的状态，不会调用 Status().Update()
	// 这样避免了中间的状态更新导致的 resourceVersion 冲突
	if err := r.reconcileValidation(ctx, kubenova); err != nil {
		logger.Error(err, "配置验证失败")
		r.setStatusPhase(kubenova, kubenovav1.PhaseFailed, fmt.Sprintf("配置验证失败: %v", err))
		// 只在需要返回错误时才更新状态，使用重试机制确保更新成功
		if updateErr := r.updateStatusWithRetry(ctx, kubenova); updateErr != nil {
			logger.Error(updateErr, "更新状态失败")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// ========== 阶段 2: 检查 Namespace 是否存在 ==========
	if err := r.reconcileNamespace(ctx, kubenova); err != nil {
		logger.Error(err, "Namespace 不存在或无法访问")
		r.setStatusPhase(kubenova, kubenovav1.PhaseFailed, fmt.Sprintf("Namespace 错误: %v", err))
		if updateErr := r.updateStatusWithRetry(ctx, kubenova); updateErr != nil {
			logger.Error(updateErr, "更新状态失败")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// ========== 阶段 3: 创建 RBAC ==========
	if err := r.reconcileRBAC(ctx, kubenova); err != nil {
		logger.Error(err, "创建 RBAC 资源失败")
		r.setStatusPhase(kubenova, kubenovav1.PhaseFailed, fmt.Sprintf("创建 RBAC 资源失败: %v", err))
		if updateErr := r.updateStatusWithRetry(ctx, kubenova); updateErr != nil {
			logger.Error(updateErr, "更新状态失败")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// ========== 阶段 4: 创建 Secret ==========
	if err := r.reconcileSecret(ctx, kubenova); err != nil {
		logger.Error(err, "创建 Secret 失败")
		r.setStatusPhase(kubenova, kubenovav1.PhaseFailed, fmt.Sprintf("创建 Secret 失败: %v", err))
		if updateErr := r.updateStatusWithRetry(ctx, kubenova); updateErr != nil {
			logger.Error(updateErr, "更新状态失败")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// ========== 阶段 5: 创建 ConfigMap ==========
	if err := r.reconcileConfigMaps(ctx, kubenova); err != nil {
		logger.Error(err, "创建 ConfigMap 失败")
		r.setStatusPhase(kubenova, kubenovav1.PhaseFailed, fmt.Sprintf("创建 ConfigMap 失败: %v", err))
		if updateErr := r.updateStatusWithRetry(ctx, kubenova); updateErr != nil {
			logger.Error(updateErr, "更新状态失败")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// ========== 阶段 6: 部署后端服务 ==========
	if err := r.reconcileServices(ctx, kubenova); err != nil {
		logger.Error(err, "部署后端服务失败")
		r.setStatusPhase(kubenova, kubenovav1.PhaseFailed, fmt.Sprintf("部署后端服务失败: %v", err))
		if updateErr := r.updateStatusWithRetry(ctx, kubenova); updateErr != nil {
			logger.Error(updateErr, "更新状态失败")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// ========== 阶段 7: 部署 Web 前端 ==========
	if err := r.reconcileWeb(ctx, kubenova); err != nil {
		logger.Error(err, "部署 Web 前端失败")
		r.setStatusPhase(kubenova, kubenovav1.PhaseFailed, fmt.Sprintf("部署 Web 前端失败: %v", err))
		if updateErr := r.updateStatusWithRetry(ctx, kubenova); updateErr != nil {
			logger.Error(updateErr, "更新状态失败")
		}
		return ctrl.Result{RequeueAfter: time.Minute}, err
	}

	// ========== 阶段 8: 检查组件状态 ==========
	// checkComponentStatus 会在函数内部调用 updateStatusWithRetry，统一更新所有状态
	if err := r.checkComponentStatus(ctx, kubenova); err != nil {
		logger.Error(err, "检查组件状态失败")
		// 即使状态更新失败，也继续重试 reconcile，不返回 error
		// 因为资源本身已经创建成功，只是状态更新失败而已
		return ctrl.Result{RequeueAfter: 30 * time.Second}, nil
	}

	logger.Info("KubeNova 协调完成")
	return ctrl.Result{RequeueAfter: 5 * time.Minute}, nil
}

// updateStatusWithRetry 带重试机制的状态更新函数
// 这是解决 resourceVersion 冲突的关键方法
//
// 工作原理：
// 1. 重新从 API Server 获取最新版本的对象（带最新的 resourceVersion）
// 2. 将我们修改的状态复制到最新对象上
// 3. 尝试更新，如果冲突则重试
// 4. 使用指数退避策略避免过于频繁的重试
func (r *KubeNovaReconciler) updateStatusWithRetry(ctx context.Context, kubenova *kubenovav1.KubeNova) error {
	logger := log.FromContext(ctx)

	// 最多重试 3 次，对于大多数场景足够了
	for i := 0; i < 3; i++ {
		// 关键步骤：重新获取最新版本的对象
		// 这样我们就能得到最新的 resourceVersion
		latest := &kubenovav1.KubeNova{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      kubenova.Name,
			Namespace: kubenova.Namespace,
		}, latest); err != nil {
			logger.Error(err, "重新获取对象失败", "重试次数", i+1)
			return err
		}

		// 将我们在内存中修改的状态复制到最新对象上
		// 这样既保留了最新的 resourceVersion，又有了我们想要的状态修改
		latest.Status = kubenova.Status

		// 尝试更新状态
		if err := r.Status().Update(ctx, latest); err != nil {
			// 如果是冲突错误且还有重试机会，继续重试
			if errors.IsConflict(err) && i < 2 {
				logger.Info("状态更新冲突，准备重试", "重试次数", i+1)
				// 指数退避：第一次等待 100ms，第二次等待 200ms
				time.Sleep(time.Millisecond * 100 * time.Duration(i+1))
				continue
			}
			// 其他错误或重试次数用尽
			logger.Error(err, "状态更新失败", "重试次数", i+1)
			return err
		}

		// 成功更新！将最新对象的状态和 resourceVersion 复制回原对象
		// 这样调用者持有的对象也是最新的了
		kubenova.Status = latest.Status
		kubenova.ResourceVersion = latest.ResourceVersion
		logger.Info("状态更新成功", "重试次数", i+1)
		return nil
	}

	return fmt.Errorf("状态更新失败，已重试 3 次")
}

// reconcileDelete 处理删除逻辑
func (r *KubeNovaReconciler) reconcileDelete(ctx context.Context, kubenova *kubenovav1.KubeNova) (ctrl.Result, error) {
	logger := log.FromContext(ctx)
	logger.Info("开始删除 KubeNova 资源")

	r.setStatusPhase(kubenova, kubenovav1.PhaseDeleting, "正在删除资源")
	// 使用重试机制更新删除状态
	_ = r.updateStatusWithRetry(ctx, kubenova)

	// 只删除 ClusterRoleBinding（因为它是集群级资源）
	// 其他命名空间级资源会通过 OwnerReference 自动删除
	crbName := fmt.Sprintf("kube-nova-%s-%s-cluster-admin", kubenova.Namespace, kubenova.Name)
	crb := &rbacv1.ClusterRoleBinding{}
	if err := r.Get(ctx, types.NamespacedName{Name: crbName}, crb); err == nil {
		logger.Info("删除 ClusterRoleBinding", "名称", crbName)
		if err := r.Delete(ctx, crb); err != nil && !errors.IsNotFound(err) {
			logger.Error(err, "删除 ClusterRoleBinding 失败")
			return ctrl.Result{}, err
		}
	}

	// 移除 Finalizer，允许对象被真正删除
	logger.Info("移除 Finalizer")
	controllerutil.RemoveFinalizer(kubenova, kubenovaFinalizer)
	if err := r.Update(ctx, kubenova); err != nil {
		logger.Error(err, "移除 Finalizer 失败")
		return ctrl.Result{}, err
	}

	logger.Info("KubeNova 资源删除完成")
	return ctrl.Result{}, nil
}

// reconcileValidation 验证配置
// 注意：此函数只修改内存中的状态，不调用 Status().Update()
func (r *KubeNovaReconciler) reconcileValidation(ctx context.Context, kubenova *kubenovav1.KubeNova) error {
	logger := log.FromContext(ctx)
	logger.Info("开始验证配置")

	// 在内存中设置状态，不立即持久化
	r.setStatusPhase(kubenova, kubenovav1.PhaseValidating, "正在验证配置")

	if err := validator.ValidateKubeNova(kubenova); err != nil {
		// 记录验证失败的 Condition
		meta.SetStatusCondition(&kubenova.Status.Conditions, metav1.Condition{
			Type:               kubenovav1.ConditionTypeValidated,
			Status:             metav1.ConditionFalse,
			Reason:             "ValidationFailed",
			Message:            fmt.Sprintf("配置验证失败: %v", err),
			ObservedGeneration: kubenova.Generation,
		})
		// 不在这里调用 Status().Update()，让调用者统一处理
		return err
	}

	// 记录验证成功的 Condition
	meta.SetStatusCondition(&kubenova.Status.Conditions, metav1.Condition{
		Type:               kubenovav1.ConditionTypeValidated,
		Status:             metav1.ConditionTrue,
		Reason:             "ValidationSucceeded",
		Message:            "配置验证成功",
		ObservedGeneration: kubenova.Generation,
	})
	// 同样不在这里调用 Status().Update()

	logger.Info("配置验证完成")
	return nil
}

// reconcileNamespace 检查 Namespace 是否存在
func (r *KubeNovaReconciler) reconcileNamespace(ctx context.Context, kubenova *kubenovav1.KubeNova) error {
	logger := log.FromContext(ctx)
	namespace := kubenova.Namespace

	ns := &corev1.Namespace{}
	if err := r.Get(ctx, types.NamespacedName{Name: namespace}, ns); err != nil {
		if errors.IsNotFound(err) {
			return fmt.Errorf("命名空间 %s 不存在，请先创建命名空间", namespace)
		}
		return fmt.Errorf("获取命名空间失败: %w", err)
	}

	logger.Info("命名空间检查通过", "命名空间", namespace)
	return nil
}

// reconcileRBAC 创建 RBAC 资源
func (r *KubeNovaReconciler) reconcileRBAC(ctx context.Context, kubenova *kubenovav1.KubeNova) error {
	logger := log.FromContext(ctx)
	logger.Info("开始创建 RBAC 资源")

	namespace := kubenova.Namespace

	// 创建 ServiceAccount
	sa := builder.BuildServiceAccount(kubenova, namespace)
	if err := controllerutil.SetControllerReference(kubenova, sa, r.Scheme); err != nil {
		return fmt.Errorf("设置 ServiceAccount OwnerReference 失败: %w", err)
	}

	existingSA := &corev1.ServiceAccount{}
	if err := r.Get(ctx, types.NamespacedName{Name: sa.Name, Namespace: namespace}, existingSA); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("创建 ServiceAccount", "名称", sa.Name)
			if err := r.Create(ctx, sa); err != nil {
				return fmt.Errorf("创建 ServiceAccount 失败: %w", err)
			}
		} else {
			return fmt.Errorf("获取 ServiceAccount 失败: %w", err)
		}
	}

	// 创建 ClusterRoleBinding
	crb := builder.BuildClusterRoleBinding(kubenova, namespace)

	existingCRB := &rbacv1.ClusterRoleBinding{}
	if err := r.Get(ctx, types.NamespacedName{Name: crb.Name}, existingCRB); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("创建 ClusterRoleBinding", "名称", crb.Name)
			if err := r.Create(ctx, crb); err != nil {
				return fmt.Errorf("创建 ClusterRoleBinding 失败: %w", err)
			}
		} else {
			return fmt.Errorf("获取 ClusterRoleBinding 失败: %w", err)
		}
	}

	logger.Info("RBAC 资源创建完成")
	return nil
}

// reconcileSecret 创建 Secret
func (r *KubeNovaReconciler) reconcileSecret(ctx context.Context, kubenova *kubenovav1.KubeNova) error {
	logger := log.FromContext(ctx)
	logger.Info("开始创建 Secret")

	namespace := kubenova.Namespace

	// 获取 Node IP 和 NodePort（用于 MinIO 代理端点）
	nodeIP := r.getNodeIP(ctx)
	nodePort := r.getWebNodePort(ctx, kubenova, namespace)

	secret := builder.BuildSecret(kubenova, namespace, nodeIP, nodePort)

	if err := controllerutil.SetControllerReference(kubenova, secret, r.Scheme); err != nil {
		return fmt.Errorf("设置 OwnerReference 失败: %w", err)
	}

	existingSecret := &corev1.Secret{}
	if err := r.Get(ctx, types.NamespacedName{Name: secret.Name, Namespace: namespace}, existingSecret); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("创建 Secret", "名称", secret.Name)
			if err := r.Create(ctx, secret); err != nil {
				return fmt.Errorf("创建 Secret 失败: %w", err)
			}
		} else {
			return fmt.Errorf("获取 Secret 失败: %w", err)
		}
	} else {
		// 只在 Data 有变化时才更新
		if !reflect.DeepEqual(existingSecret.Data, secret.Data) {
			existingSecret.Data = secret.Data
			logger.Info("更新 Secret", "名称", secret.Name)
			if err := r.Update(ctx, existingSecret); err != nil {
				return fmt.Errorf("更新 Secret 失败: %w", err)
			}
		}
	}

	return nil
}

// getWebNodePort 获取 Web Service 的 NodePort
func (r *KubeNovaReconciler) getWebNodePort(ctx context.Context, kubenova *kubenovav1.KubeNova, namespace string) int32 {
	logger := log.FromContext(ctx)

	// 如果不是 NodePort 模式，返回 0
	if kubenova.Spec.Web.ExposeType != "nodeport" {
		return 0
	}

	// 尝试获取 Web Service
	webSvc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      "kube-nova-web",
		Namespace: namespace,
	}, webSvc); err != nil {
		logger.Error(err, "获取 Web Service 失败，返回默认端口")
		// 如果用户配置了端口，返回用户配置
		if kubenova.Spec.Web.NodePort != nil && kubenova.Spec.Web.NodePort.HTTPPort > 0 {
			return kubenova.Spec.Web.NodePort.HTTPPort
		}
		// 否则返回默认端口
		return 30080
	}

	// 优先返回 HTTPS 端口（如果启用）
	if kubenova.Spec.Web.NodePort != nil &&
		kubenova.Spec.Web.NodePort.HTTPS != nil &&
		kubenova.Spec.Web.NodePort.HTTPS.Enabled {
		for _, port := range webSvc.Spec.Ports {
			if port.Name == "https" && port.NodePort > 0 {
				return port.NodePort
			}
		}
	}

	// 返回 HTTP 端口
	for _, port := range webSvc.Spec.Ports {
		if port.Name == "http" && port.NodePort > 0 {
			return port.NodePort
		}
	}

	// 如果都没找到，返回默认值
	if kubenova.Spec.Web.NodePort != nil && kubenova.Spec.Web.NodePort.HTTPPort > 0 {
		return kubenova.Spec.Web.NodePort.HTTPPort
	}
	return 30080
}

// reconcileConfigMaps 创建所有 ConfigMap
func (r *KubeNovaReconciler) reconcileConfigMaps(ctx context.Context, kubenova *kubenovav1.KubeNova) error {
	logger := log.FromContext(ctx)
	logger.Info("开始创建 ConfigMap")

	namespace := kubenova.Namespace
	configMaps := builder.BuildAllConfigMaps(kubenova, namespace)

	for _, cm := range configMaps {
		if err := controllerutil.SetControllerReference(kubenova, cm, r.Scheme); err != nil {
			return fmt.Errorf("设置 OwnerReference 失败: %w", err)
		}

		existingCM := &corev1.ConfigMap{}
		if err := r.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: namespace}, existingCM); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("创建 ConfigMap", "名称", cm.Name)
				if err := r.Create(ctx, cm); err != nil {
					return fmt.Errorf("创建 ConfigMap %s 失败: %w", cm.Name, err)
				}
			} else {
				return fmt.Errorf("获取 ConfigMap 失败: %w", err)
			}
		} else {
			// 只在 Data 有变化时才更新
			if !reflect.DeepEqual(existingCM.Data, cm.Data) {
				existingCM.Data = cm.Data
				logger.Info("更新 ConfigMap", "名称", cm.Name)
				if err := r.Update(ctx, existingCM); err != nil {
					return fmt.Errorf("更新 ConfigMap %s 失败: %w", cm.Name, err)
				}
			}
		}
	}

	return nil
}

// reconcileServices 部署所有后端服务
func (r *KubeNovaReconciler) reconcileServices(ctx context.Context, kubenova *kubenovav1.KubeNova) error {
	logger := log.FromContext(ctx)
	logger.Info("开始部署后端服务")

	r.setStatusPhase(kubenova, kubenovav1.PhaseCreating, "正在部署后端服务")

	namespace := kubenova.Namespace
	services := builder.BuildAllServices(kubenova, namespace)

	for serviceName, resources := range services {
		// 部署 Deployment
		deployment := resources.Deployment
		if err := controllerutil.SetControllerReference(kubenova, deployment, r.Scheme); err != nil {
			return fmt.Errorf("设置 OwnerReference 失败: %w", err)
		}

		existingDeploy := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: namespace}, existingDeploy); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("创建 Deployment", "服务", serviceName, "名称", deployment.Name)
				if err := r.Create(ctx, deployment); err != nil {
					return fmt.Errorf("创建 Deployment %s 失败: %w", serviceName, err)
				}
			} else {
				return fmt.Errorf("获取 Deployment 失败: %w", err)
			}
		} else {
			// 只在 Spec 有实质变化时才更新
			if !deploymentSpecEqual(&existingDeploy.Spec, &deployment.Spec) {
				// 关键修复：重新获取最新对象，避免 resourceVersion 冲突
				// 这是处理普通资源（非状态）更新冲突的标准方法
				latestDeploy := &appsv1.Deployment{}
				if err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: namespace}, latestDeploy); err != nil {
					return fmt.Errorf("重新获取 Deployment 失败: %w", err)
				}

				// 更新 Spec
				latestDeploy.Spec.Replicas = deployment.Spec.Replicas
				latestDeploy.Spec.Template.Spec.Containers = deployment.Spec.Template.Spec.Containers
				latestDeploy.Spec.Template.Spec.Volumes = deployment.Spec.Template.Spec.Volumes
				latestDeploy.Spec.Template.Spec.ServiceAccountName = deployment.Spec.Template.Spec.ServiceAccountName
				latestDeploy.Spec.Template.Spec.ImagePullSecrets = deployment.Spec.Template.Spec.ImagePullSecrets

				logger.Info("更新 Deployment", "服务", serviceName, "名称", deployment.Name)
				if err := r.Update(ctx, latestDeploy); err != nil {
					return fmt.Errorf("更新 Deployment %s 失败: %w", serviceName, err)
				}
			}
		}

		// 部署 Service
		service := resources.Service
		if err := controllerutil.SetControllerReference(kubenova, service, r.Scheme); err != nil {
			return fmt.Errorf("设置 OwnerReference 失败: %w", err)
		}

		existingSvc := &corev1.Service{}
		if err := r.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: namespace}, existingSvc); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("创建 Service", "服务", serviceName, "名称", service.Name)
				if err := r.Create(ctx, service); err != nil {
					return fmt.Errorf("创建 Service %s 失败: %w", serviceName, err)
				}
			} else {
				return fmt.Errorf("获取 Service 失败: %w", err)
			}
		}
	}

	logger.Info("后端服务部署完成")
	return nil
}

// reconcileWeb 部署 Web 前端
func (r *KubeNovaReconciler) reconcileWeb(ctx context.Context, kubenova *kubenovav1.KubeNova) error {
	logger := log.FromContext(ctx)
	logger.Info("开始部署 Web 前端")

	namespace := kubenova.Namespace
	webResources := builder.BuildWebResources(kubenova, namespace)

	// 部署 Nginx ConfigMap
	if webResources.NginxConfigMap != nil {
		cm := webResources.NginxConfigMap
		if err := controllerutil.SetControllerReference(kubenova, cm, r.Scheme); err != nil {
			return fmt.Errorf("设置 OwnerReference 失败: %w", err)
		}

		existingCM := &corev1.ConfigMap{}
		if err := r.Get(ctx, types.NamespacedName{Name: cm.Name, Namespace: namespace}, existingCM); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("创建 Web ConfigMap", "名称", cm.Name)
				if err := r.Create(ctx, cm); err != nil {
					return fmt.Errorf("创建 Web ConfigMap 失败: %w", err)
				}
			} else {
				return fmt.Errorf("获取 Web ConfigMap 失败: %w", err)
			}
		} else {
			// 只在 Data 有变化时才更新
			if !reflect.DeepEqual(existingCM.Data, cm.Data) {
				existingCM.Data = cm.Data
				logger.Info("更新 Web ConfigMap", "名称", cm.Name)
				if err := r.Update(ctx, existingCM); err != nil {
					return fmt.Errorf("更新 Web ConfigMap 失败: %w", err)
				}
			}
		}
	}

	// 部署 Deployment
	deployment := webResources.Deployment
	if err := controllerutil.SetControllerReference(kubenova, deployment, r.Scheme); err != nil {
		return fmt.Errorf("设置 OwnerReference 失败: %w", err)
	}

	existingDeploy := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: namespace}, existingDeploy); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("创建 Web Deployment", "名称", deployment.Name)
			if err := r.Create(ctx, deployment); err != nil {
				return fmt.Errorf("创建 Web Deployment 失败: %w", err)
			}
		} else {
			return fmt.Errorf("获取 Web Deployment 失败: %w", err)
		}
	} else {
		// 只在 Spec 有实质变化时才更新
		if !deploymentSpecEqual(&existingDeploy.Spec, &deployment.Spec) {
			latestDeploy := &appsv1.Deployment{}
			if err := r.Get(ctx, types.NamespacedName{Name: deployment.Name, Namespace: namespace}, latestDeploy); err != nil {
				return fmt.Errorf("重新获取 Web Deployment 失败: %w", err)
			}

			// 更新 Spec
			latestDeploy.Spec.Replicas = deployment.Spec.Replicas
			latestDeploy.Spec.Template.Spec.Containers = deployment.Spec.Template.Spec.Containers
			latestDeploy.Spec.Template.Spec.Volumes = deployment.Spec.Template.Spec.Volumes
			latestDeploy.Spec.Template.Spec.ImagePullSecrets = deployment.Spec.Template.Spec.ImagePullSecrets

			logger.Info("更新 Web Deployment", "名称", deployment.Name)
			if err := r.Update(ctx, latestDeploy); err != nil {
				return fmt.Errorf("更新 Web Deployment 失败: %w", err)
			}
		}
	}

	// 部署 Service
	service := webResources.Service
	if err := controllerutil.SetControllerReference(kubenova, service, r.Scheme); err != nil {
		return fmt.Errorf("设置 OwnerReference 失败: %w", err)
	}

	existingSvc := &corev1.Service{}
	if err := r.Get(ctx, types.NamespacedName{Name: service.Name, Namespace: namespace}, existingSvc); err != nil {
		if errors.IsNotFound(err) {
			logger.Info("创建 Web Service", "名称", service.Name)
			if err := r.Create(ctx, service); err != nil {
				return fmt.Errorf("创建 Web Service 失败: %w", err)
			}
		} else {
			return fmt.Errorf("获取 Web Service 失败: %w", err)
		}
	}

	// 部署 Ingress
	if webResources.Ingress != nil {
		ingress := webResources.Ingress
		if err := controllerutil.SetControllerReference(kubenova, ingress, r.Scheme); err != nil {
			return fmt.Errorf("设置 OwnerReference 失败: %w", err)
		}

		existingIngress := &networkingv1.Ingress{}
		if err := r.Get(ctx, types.NamespacedName{Name: ingress.Name, Namespace: namespace}, existingIngress); err != nil {
			if errors.IsNotFound(err) {
				logger.Info("创建 Ingress", "名称", ingress.Name)
				if err := r.Create(ctx, ingress); err != nil {
					return fmt.Errorf("创建 Ingress 失败: %w", err)
				}
			} else {
				return fmt.Errorf("获取 Ingress 失败: %w", err)
			}
		}
	}

	logger.Info("Web 前端部署完成")
	return nil
}

// deploymentSpecEqual 比较 Deployment Spec
func deploymentSpecEqual(existing, desired *appsv1.DeploymentSpec) bool {
	if !compareInt32Ptr(existing.Replicas, desired.Replicas) {
		return false
	}
	if len(existing.Template.Spec.Containers) != len(desired.Template.Spec.Containers) {
		return false
	}
	if len(existing.Template.Spec.Containers) > 0 && len(desired.Template.Spec.Containers) > 0 {
		existingContainer := existing.Template.Spec.Containers[0]
		desiredContainer := desired.Template.Spec.Containers[0]
		if existingContainer.Image != desiredContainer.Image {
			return false
		}
		if !compareEnvVars(existingContainer.Env, desiredContainer.Env) {
			return false
		}
		if !compareResourceRequirements(existingContainer.Resources, desiredContainer.Resources) {
			return false
		}
		if !compareVolumeMounts(existingContainer.VolumeMounts, desiredContainer.VolumeMounts) {
			return false
		}
	}
	if !compareVolumes(existing.Template.Spec.Volumes, desired.Template.Spec.Volumes) {
		return false
	}
	if existing.Template.Spec.ServiceAccountName != desired.Template.Spec.ServiceAccountName {
		return false
	}
	return true
}

func compareInt32Ptr(a, b *int32) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func compareEnvVars(a, b []corev1.EnvVar) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]string)
	for _, env := range a {
		if env.Value != "" {
			aMap[env.Name] = env.Value
		}
	}
	bMap := make(map[string]string)
	for _, env := range b {
		if env.Value != "" {
			bMap[env.Name] = env.Value
		}
	}
	return reflect.DeepEqual(aMap, bMap)
}

func compareResourceRequirements(a, b corev1.ResourceRequirements) bool {
	return reflect.DeepEqual(a.Requests, b.Requests) && reflect.DeepEqual(a.Limits, b.Limits)
}

func compareVolumeMounts(a, b []corev1.VolumeMount) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]corev1.VolumeMount)
	for _, vm := range a {
		aMap[vm.Name] = vm
	}
	for _, vm := range b {
		if existingVM, ok := aMap[vm.Name]; !ok || existingVM.MountPath != vm.MountPath {
			return false
		}
	}
	return true
}

func compareVolumes(a, b []corev1.Volume) bool {
	if len(a) != len(b) {
		return false
	}
	aMap := make(map[string]bool)
	for _, v := range a {
		aMap[v.Name] = true
	}
	for _, v := range b {
		if !aMap[v.Name] {
			return false
		}
	}
	return true
}

// checkComponentStatus 检查所有组件状态并统一更新
func (r *KubeNovaReconciler) checkComponentStatus(ctx context.Context, kubenova *kubenovav1.KubeNova) error {
	logger := log.FromContext(ctx)
	logger.Info("检查组件状态")

	namespace := kubenova.Namespace

	if kubenova.Status.ComponentStatus.Services == nil {
		kubenova.Status.ComponentStatus.Services = make(map[string]*kubenovav1.ComponentStatus)
	}

	allReady := true

	services := []string{"portal-api", "portal-rpc", "manager-api", "manager-rpc",
		"workload-api", "console-api", "console-rpc"}

	for _, serviceName := range services {
		deployment := &appsv1.Deployment{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      serviceName,
			Namespace: namespace,
		}, deployment); err != nil {
			if !errors.IsNotFound(err) {
				logger.Error(err, "获取 Deployment 状态失败", "服务", serviceName)
			}
			continue
		}

		status := &kubenovav1.ComponentStatus{
			DesiredReplicas:    *deployment.Spec.Replicas,
			ReadyReplicas:      deployment.Status.ReadyReplicas,
			LastTransitionTime: metav1.Now(),
		}

		if deployment.Status.ReadyReplicas == *deployment.Spec.Replicas {
			status.State = kubenovav1.ComponentStateReady
			status.Message = "服务运行正常"
		} else if deployment.Status.ReadyReplicas > 0 {
			status.State = kubenovav1.ComponentStateRunning
			status.Message = fmt.Sprintf("服务部分就绪 (%d/%d)", deployment.Status.ReadyReplicas, *deployment.Spec.Replicas)
			allReady = false
		} else {
			status.State = kubenovav1.ComponentStatePending
			status.Message = "等待 Pod 启动"
			allReady = false
		}

		kubenova.Status.ComponentStatus.Services[serviceName] = status
	}

	// 检查 Web 状态
	webDeployment := &appsv1.Deployment{}
	if err := r.Get(ctx, types.NamespacedName{
		Name:      "kube-nova-web",
		Namespace: namespace,
	}, webDeployment); err == nil {
		status := &kubenovav1.ComponentStatus{
			DesiredReplicas:    *webDeployment.Spec.Replicas,
			ReadyReplicas:      webDeployment.Status.ReadyReplicas,
			LastTransitionTime: metav1.Now(),
		}

		if webDeployment.Status.ReadyReplicas == *webDeployment.Spec.Replicas {
			status.State = kubenovav1.ComponentStateReady
			status.Message = "Web 前端运行正常"
		} else {
			status.State = kubenovav1.ComponentStateRunning
			status.Message = fmt.Sprintf("Web 前端部分就绪 (%d/%d)",
				webDeployment.Status.ReadyReplicas, *webDeployment.Spec.Replicas)
			allReady = false
		}

		kubenova.Status.ComponentStatus.Web = status
	}

	// 更新访问信息
	r.updateAccessInfo(ctx, kubenova, namespace)

	// 更新整体状态
	if allReady {
		r.setStatusPhase(kubenova, kubenovav1.PhaseReady, "所有组件运行正常")
		meta.SetStatusCondition(&kubenova.Status.Conditions, metav1.Condition{
			Type:               kubenovav1.ConditionTypeReady,
			Status:             metav1.ConditionTrue,
			Reason:             "AllComponentsReady",
			Message:            "所有组件运行正常",
			ObservedGeneration: kubenova.Generation,
		})
	} else {
		r.setStatusPhase(kubenova, kubenovav1.PhaseCreating, "部分组件尚未就绪")
		meta.SetStatusCondition(&kubenova.Status.Conditions, metav1.Condition{
			Type:               kubenovav1.ConditionTypeReady,
			Status:             metav1.ConditionFalse,
			Reason:             "ComponentsNotReady",
			Message:            "部分组件尚未就绪",
			ObservedGeneration: kubenova.Generation,
		})
	}

	return r.updateStatusWithRetry(ctx, kubenova)
}

func (r *KubeNovaReconciler) updateAccessInfo(ctx context.Context, kubenova *kubenovav1.KubeNova, namespace string) {
	logger := log.FromContext(ctx)

	if kubenova.Status.AccessInfo == nil {
		kubenova.Status.AccessInfo = &kubenovav1.AccessInfo{}
	}

	// 更新 Web URL
	if kubenova.Spec.Web.ExposeType == "ingress" && kubenova.Spec.Web.Ingress != nil {
		protocol := "http"
		if kubenova.Spec.Web.Ingress.TLS != nil && kubenova.Spec.Web.Ingress.TLS.Enabled {
			protocol = "https"
		}
		kubenova.Status.AccessInfo.WebURL = fmt.Sprintf("%s://%s", protocol, kubenova.Spec.Web.Ingress.Host)
	} else if kubenova.Spec.Web.ExposeType == "nodeport" {
		webSvc := &corev1.Service{}
		if err := r.Get(ctx, types.NamespacedName{
			Name:      "kube-nova-web",
			Namespace: namespace,
		}, webSvc); err == nil {
			var httpPort, httpsPort int32
			for _, port := range webSvc.Spec.Ports {
				if port.Name == "http" && port.NodePort > 0 {
					httpPort = port.NodePort
				}
				if port.Name == "https" && port.NodePort > 0 {
					httpsPort = port.NodePort
				}
			}

			nodeIP := r.getNodeIP(ctx)

			if httpsPort > 0 {
				if nodeIP != "" {
					kubenova.Status.AccessInfo.WebURL = fmt.Sprintf("https://%s:%d", nodeIP, httpsPort)
				} else {
					kubenova.Status.AccessInfo.WebURL = fmt.Sprintf("https://<NODE-IP>:%d", httpsPort)
				}
			} else if httpPort > 0 {
				if nodeIP != "" {
					kubenova.Status.AccessInfo.WebURL = fmt.Sprintf("http://%s:%d", nodeIP, httpPort)
				} else {
					kubenova.Status.AccessInfo.WebURL = fmt.Sprintf("http://<NODE-IP>:%d", httpPort)
				}
			} else {
				kubenova.Status.AccessInfo.WebURL = "NodePort (端口分配中...)"
			}
		} else {
			logger.Error(err, "获取 Web Service 失败")
			kubenova.Status.AccessInfo.WebURL = "Service 创建中..."
		}
	}

	// 更新服务端点
	if kubenova.Status.AccessInfo.ServiceEndpoints == nil {
		kubenova.Status.AccessInfo.ServiceEndpoints = make(map[string]string)
	}

	services := []string{"portal-api", "portal-rpc", "manager-api", "manager-rpc",
		"workload-api", "console-api", "console-rpc"}
	for _, svc := range services {
		kubenova.Status.AccessInfo.ServiceEndpoints[svc] = fmt.Sprintf("%s.%s.svc.cluster.local", svc, namespace)
	}

	kubenova.Status.AccessInfo.DatabaseEndpoint = kubenova.Spec.Database.GetDatabaseEndpoint()
	kubenova.Status.AccessInfo.CacheEndpoint = kubenova.Spec.Cache.GetCacheEndpoint()
	kubenova.Status.AccessInfo.StorageEndpoint = kubenova.Spec.Storage.GetStorageEndpoint()

	if kubenova.IsTelemetryEnabled() {
		kubenova.Status.AccessInfo.JaegerUIURL = kubenova.Spec.Telemetry.JaegerEndpoint
	}
}

func (r *KubeNovaReconciler) getNodeIP(ctx context.Context) string {
	logger := log.FromContext(ctx)

	nodeList := &corev1.NodeList{}
	if err := r.List(ctx, nodeList); err != nil {
		logger.Error(err, "获取 Node 列表失败")
		return ""
	}

	if len(nodeList.Items) == 0 {
		return ""
	}

	for _, node := range nodeList.Items {
		isReady := false
		for _, condition := range node.Status.Conditions {
			if condition.Type == corev1.NodeReady && condition.Status == corev1.ConditionTrue {
				isReady = true
				break
			}
		}

		if !isReady {
			continue
		}

		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeExternalIP {
				return addr.Address
			}
		}

		for _, addr := range node.Status.Addresses {
			if addr.Type == corev1.NodeInternalIP {
				return addr.Address
			}
		}
	}

	if len(nodeList.Items) > 0 {
		for _, addr := range nodeList.Items[0].Status.Addresses {
			if addr.Type == corev1.NodeExternalIP || addr.Type == corev1.NodeInternalIP {
				return addr.Address
			}
		}
	}

	return ""
}

// setStatusPhase 只修改内存中的状态对象，不立即持久化
func (r *KubeNovaReconciler) setStatusPhase(kubenova *kubenovav1.KubeNova, phase kubenovav1.DeploymentPhase, message string) {
	kubenova.Status.Phase = phase
	kubenova.Status.Message = message
	kubenova.Status.LastUpdateTime = metav1.Now()
	kubenova.Status.ObservedGeneration = kubenova.Generation
}

func (r *KubeNovaReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&kubenovav1.KubeNova{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Owns(&corev1.ConfigMap{}).
		Owns(&corev1.Secret{}).
		Owns(&corev1.ServiceAccount{}).
		Owns(&networkingv1.Ingress{}).
		Complete(r)
}
