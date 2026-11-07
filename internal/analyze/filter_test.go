package analyze

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
)

// TestFilterPod는 --ignore-daemonsets --delete-emptydir-data를 사용하고
// --force는 사용하지 않는 조건에서 Issue #6의 분류와 검사 순서를 확인합니다.
// DaemonSet은 존재한다고 가정하며 API 조회 없이 공통 도우미로 입력을 만듭니다.
func TestFilterPod(t *testing.T) {
	tests := []struct {
		name     string
		pod      corev1.Pod
		want     Verdict
		wantDone bool
	}{
		{
			name: "apps/v1 DaemonSet Pod는 건너뜀",
			pod: newPod("kube-system", "agent",
				withController("apps/v1", "DaemonSet", "agent")),
			want:     Verdict{Namespace: "kube-system", Name: "agent", Result: Skipped, Reason: "DaemonSet (--ignore-daemonsets)"},
			wantDone: true,
		},
		{
			name: "mirror Pod는 건너뜀",
			pod: newPod("kube-system", "static",
				withOwner("v1", "Node", "worker-1"), withMirrorAnnotation()),
			want:     Verdict{Namespace: "kube-system", Name: "static", Result: Skipped, Reason: "mirror Pod"},
			wantDone: true,
		},
		{
			name:     "컨트롤러 없는 Pod는 force 필요",
			pod:      newPod("ops", "debug"),
			want:     Verdict{Namespace: "ops", Name: "debug", Result: NeedsFlag, Reason: "no controller (--force)"},
			wantDone: true,
		},
		{
			name: "ReplicaSet Pod는 다음 검사로 넘김",
			pod: newPod("shop", "web",
				withController("apps/v1", "ReplicaSet", "web")),
		},
		{
			name:     "ownerReferences 없는 mirror Pod도 컨트롤러 검사 전에 건너뜀",
			pod:      newPod("kube-system", "static-orphan", withMirrorAnnotation()),
			want:     Verdict{Namespace: "kube-system", Name: "static-orphan", Result: Skipped, Reason: "mirror Pod"},
			wantDone: true,
		},
		{
			name: "controller 표시 없는 소유자 참조만 있으면 force 필요",
			pod: newPod("ops", "owned",
				withOwner("apps/v1", "ReplicaSet", "owned")),
			want:     Verdict{Namespace: "ops", Name: "owned", Result: NeedsFlag, Reason: "no controller (--force)"},
			wantDone: true,
		},
		{
			name: "다른 API 그룹의 DaemonSet Pod는 다음 검사로 넘김",
			pod: newPod("ops", "custom-agent",
				withController("apps.kruise.io/v1alpha1", "DaemonSet", "custom-agent")),
		},
		{
			name: "Failed DaemonSet Pod는 다음 검사로 넘김",
			pod: newPod("kube-system", "failed-agent",
				withController("apps/v1", "DaemonSet", "agent"), withPhase(corev1.PodFailed)),
		},
		{
			name: "Succeeded 상태의 컨트롤러 없는 Pod는 다음 검사로 넘김",
			pod:  newPod("batch", "report", withPhase(corev1.PodSucceeded)),
		},
		{
			name: "Succeeded mirror Pod도 건너뜀",
			pod: newPod("kube-system", "finished-static",
				withMirrorAnnotation(), withPhase(corev1.PodSucceeded)),
			want:     Verdict{Namespace: "kube-system", Name: "finished-static", Result: Skipped, Reason: "mirror Pod"},
			wantDone: true,
		},
		{
			name: "DaemonSet과 mirror 조건이 겹치면 DaemonSet 이유가 우선",
			pod: newPod("kube-system", "both",
				withController("apps/v1", "DaemonSet", "agent"), withMirrorAnnotation()),
			want:     Verdict{Namespace: "kube-system", Name: "both", Result: Skipped, Reason: "DaemonSet (--ignore-daemonsets)"},
			wantDone: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, done := filterPod(tt.pod, Snapshot{})
			if done != tt.wantDone {
				t.Errorf("filterPod() done = %t, want %t", done, tt.wantDone)
			}
			// 미확정 결과도 비교하여 다음 검사로 넘기는 경우를 evictable과 구분합니다.
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("filterPod() verdict = %#v, want %#v", got, tt.want)
			}
		})
	}
}
