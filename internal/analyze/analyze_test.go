package analyze

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestAnalyze는 Pod가 없을 때 빈 결과를 반환하고, Pod가 있으면 네임스페이스, 이름 및
// 입력 순서를 보존하면서 각 Pod에 Unknown과 "not analyzed yet"을 반환하는지 확인합니다.
func TestAnalyze(t *testing.T) {
	tests := []struct {
		name     string
		snapshot Snapshot
		want     []Verdict
	}{
		{
			name:     "Pod가 없으면 빈 결과",
			snapshot: Snapshot{},
			want:     nil,
		},
		{
			name: "Pod마다 결과 하나를 입력 순서대로 반환",
			snapshot: Snapshot{Pods: []corev1.Pod{
				newPod("shop", "web-b"),
				newPod("payment", "api-0"),
				newPod("shop", "web-a"),
			}},
			want: []Verdict{
				{Namespace: "shop", Name: "web-b", Result: Unknown, Reason: reasonNotAnalyzed},
				{Namespace: "payment", Name: "api-0", Result: Unknown, Reason: reasonNotAnalyzed},
				{Namespace: "shop", Name: "web-a", Result: Unknown, Reason: reasonNotAnalyzed},
			},
		},
		// 아래 사례는 Pod 종류, 실행 상태 및 PDB 적용 여부가 달라도 현재 임시 구현이
		// 모두 Unknown을 반환하는지 확인합니다. 실제 퇴거 허용 여부를 검사하는 테스트는 아닙니다.
		{
			name: "DaemonSet이 관리하는 Pod",
			snapshot: Snapshot{Pods: []corev1.Pod{
				newPod("kube-system", "kube-proxy-x2k9q", withController("apps/v1", "DaemonSet", "kube-proxy")),
			}},
			want: []Verdict{
				{Namespace: "kube-system", Name: "kube-proxy-x2k9q", Result: Unknown, Reason: reasonNotAnalyzed},
			},
		},
		{
			name: "mirror Pod",
			snapshot: Snapshot{Pods: []corev1.Pod{
				newPod("kube-system", "etcd-worker-1", withMirrorAnnotation()),
			}},
			want: []Verdict{
				{Namespace: "kube-system", Name: "etcd-worker-1", Result: Unknown, Reason: reasonNotAnalyzed},
			},
		},
		{
			name: "controller가 아닌 ownerReference만 있는 Pod",
			snapshot: Snapshot{Pods: []corev1.Pod{
				newPod("ops", "debug", withOwner("apps/v1", "ReplicaSet", "debug-7c9d")),
			}},
			want: []Verdict{
				{Namespace: "ops", Name: "debug", Result: Unknown, Reason: reasonNotAnalyzed},
			},
		},
		{
			name: "Succeeded 상태의 Pod",
			snapshot: Snapshot{Pods: []corev1.Pod{
				newPod("batch", "report-7xk2p", withPhase(corev1.PodSucceeded)),
			}},
			want: []Verdict{
				{Namespace: "batch", Name: "report-7xk2p", Result: Unknown, Reason: reasonNotAnalyzed},
			},
		},
		{
			name: "삭제 중인 Pod",
			snapshot: Snapshot{Pods: []corev1.Pod{
				newPod("shop", "web-6d4f9c-fghij", withDeletionTimestamp()),
			}},
			want: []Verdict{
				{Namespace: "shop", Name: "web-6d4f9c-fghij", Result: Unknown, Reason: reasonNotAnalyzed},
			},
		},
		{
			name: "PDB 하나가 매칭되는 Pod",
			snapshot: Snapshot{
				Pods: []corev1.Pod{
					newPod("shop", "web-6d4f9c-abcde",
						withController("apps/v1", "ReplicaSet", "web-6d4f9c"),
						withLabels(map[string]string{"app": "web"})),
				},
				PDBs: []policyv1.PodDisruptionBudget{
					newPDB("shop", "web", &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}),
				},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "web-6d4f9c-abcde", Result: Unknown, Reason: reasonNotAnalyzed},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Analyze(tt.snapshot)
			// Pod가 없을 때 nil과 빈 슬라이스를 모두 결과 0개로 인정하고, 결과 개수도 확인합니다.
			if len(got) != len(tt.want) {
				t.Fatalf("len(Analyze()) = %d, want %d: %#v", len(got), len(tt.want), got)
			}
			// 각 Pod의 결과 필드를 비교합니다. 오류 메시지의 %#v는 PDBs가 nil인지 빈 슬라이스인지도 보여 줍니다.
			for i := range tt.want {
				if !reflect.DeepEqual(got[i], tt.want[i]) {
					t.Errorf("Analyze()[%d] = %#v, want %#v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
