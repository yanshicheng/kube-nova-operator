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
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	kubenovav1 "github.com/yanshicheng/kube-nova-operator/api/v1"
)

// BuildServiceAccount 构建 ServiceAccount
func BuildServiceAccount(kn *kubenovav1.KubeNova, namespace string) *corev1.ServiceAccount {
	return &corev1.ServiceAccount{
		ObjectMeta: metav1.ObjectMeta{
			Name:      ServiceAccountName,
			Namespace: namespace,
			Labels: map[string]string{
				"app.ikubeops.com/name":       "kube-nova",
				"app.ikubeops.com/instance":   kn.Name,
				"app.ikubeops.com/managed-by": "kube-nova-operator",
				"app.ikubeops.com/namespace":  namespace,
				"app.ikubeops.com/component":  "rbac",
			},
		},
	}
}

// BuildClusterRoleBinding 构建 ClusterRoleBinding
func BuildClusterRoleBinding(kn *kubenovav1.KubeNova, namespace string) *rbacv1.ClusterRoleBinding {
	return &rbacv1.ClusterRoleBinding{
		ObjectMeta: metav1.ObjectMeta{
			Name: fmt.Sprintf("kube-nova-%s-%s-cluster-admin", namespace, kn.Name),
			Labels: map[string]string{
				"app.ikubeops.com/name":       "kube-nova",
				"app.ikubeops.com/instance":   kn.Name,
				"app.ikubeops.com/managed-by": "kube-nova-operator",
				"app.ikubeops.com/namespace":  namespace,
				"app.ikubeops.com/component":  "rbac",
			},
		},
		Subjects: []rbacv1.Subject{
			{
				Kind:      rbacv1.ServiceAccountKind,
				Name:      ServiceAccountName,
				Namespace: namespace,
			},
		},
		RoleRef: rbacv1.RoleRef{
			APIGroup: rbacv1.GroupName,
			Kind:     "ClusterRole",
			Name:     "cluster-admin",
		},
	}
}
