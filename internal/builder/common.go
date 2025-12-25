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
	"crypto/sha256"
	"fmt"
	"sort"
	"strings"

	corev1 "k8s.io/api/core/v1"
)

// int64Ptr 返回 int64 指针
func int64Ptr(i int64) *int64 {
	return &i
}

// boolPtr 返回 bool 指针
func boolPtr(b bool) *bool {
	return &b
}

// int32Ptr 返回 int32 指针
func int32Ptr(i int32) *int32 {
	return &i
}

// CalculateConfigMapChecksum 计算 ConfigMap 的 checksum
func CalculateConfigMapChecksum(cm *corev1.ConfigMap) string {
	if cm == nil || cm.Data == nil {
		return ""
	}

	// 对 keys 排序以保证一致性
	var keys []string
	for k := range cm.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 拼接所有数据
	var builder strings.Builder
	for _, k := range keys {
		builder.WriteString(k)
		builder.WriteString("=")
		builder.WriteString(cm.Data[k])
		builder.WriteString("\n")
	}

	// 计算 SHA256
	hash := sha256.Sum256([]byte(builder.String()))
	return fmt.Sprintf("%x", hash)
}

// CalculateSecretChecksum 计算 Secret 的 checksum
func CalculateSecretChecksum(secret *corev1.Secret) string {
	if secret == nil || secret.Data == nil {
		return ""
	}

	// 对 keys 排序以保证一致性
	var keys []string
	for k := range secret.Data {
		keys = append(keys, k)
	}
	sort.Strings(keys)

	// 拼接所有数据
	var builder strings.Builder
	for _, k := range keys {
		builder.WriteString(k)
		builder.WriteString("=")
		builder.WriteString(string(secret.Data[k]))
		builder.WriteString("\n")
	}

	// 计算 SHA256
	hash := sha256.Sum256([]byte(builder.String()))
	return fmt.Sprintf("%x", hash)
}

// CalculateConfigMapListChecksum 计算多个 ConfigMap 的总 checksum
func CalculateConfigMapListChecksum(configMaps []*corev1.ConfigMap) string {
	var checksums []string
	for _, cm := range configMaps {
		checksums = append(checksums, CalculateConfigMapChecksum(cm))
	}

	// 拼接所有 checksum
	combined := strings.Join(checksums, "|")
	hash := sha256.Sum256([]byte(combined))
	return fmt.Sprintf("%x", hash)
}
