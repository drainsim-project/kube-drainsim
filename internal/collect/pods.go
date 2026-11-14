// Package collect는 분석에 필요한 Kubernetes 객체를 조회합니다.
package collect

import (
	"context"
	"fmt"

	"github.com/drainsim-project/kube-drainsim/internal/analyze"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
)

// Pods는 대상 노드의 전체 Pod와 종료되지 않은 Pod의 소유자 DaemonSet을 조회합니다.
// PDBs는 채우지 않습니다. 호출자는 PDB 조회까지 완료한 뒤 Snapshot을 분석해야 합니다.
// 각 API 요청은 별도로 실행되므로 동일 시점의 클러스터 상태를 보장하지 않습니다.
// NotFound인 DaemonSet은 제외하고, 다른 조회 오류에는 부분 결과 대신 빈 Snapshot을 반환합니다.
func Pods(ctx context.Context, client kubernetes.Interface, nodeName string) (analyze.Snapshot, error) {
	if nodeName == "" {
		return analyze.Snapshot{}, fmt.Errorf("node name must not be empty")
	}
	pods, err := client.CoreV1().Pods(metav1.NamespaceAll).List(ctx, metav1.ListOptions{
		FieldSelector: fields.OneTermEqualSelector("spec.nodeName", nodeName).String(),
	})
	if err != nil {
		return analyze.Snapshot{}, fmt.Errorf("list pods on node %q: %w", nodeName, err)
	}
	snapshot := analyze.Snapshot{NodeName: nodeName, Pods: pods.Items}
	seen := make(map[types.NamespacedName]bool)
	for _, pod := range pods.Items {
		if pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
			continue
		}
		owner := metav1.GetControllerOf(&pod)
		if owner == nil || owner.APIVersion != appsv1.SchemeGroupVersion.String() || owner.Kind != "DaemonSet" {
			continue
		}
		key := types.NamespacedName{Namespace: pod.Namespace, Name: owner.Name}
		if seen[key] {
			continue
		}
		seen[key] = true
		ds, err := client.AppsV1().DaemonSets(key.Namespace).Get(ctx, key.Name, metav1.GetOptions{})
		if apierrors.IsNotFound(err) {
			continue
		}
		if err != nil {
			return analyze.Snapshot{}, fmt.Errorf("get DaemonSet %s: %w", key, err)
		}
		snapshot.DaemonSets = append(snapshot.DaemonSets, *ds)
	}
	return snapshot, nil
}
