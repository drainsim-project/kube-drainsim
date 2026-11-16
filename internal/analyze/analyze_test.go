package analyze

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestAnalyze는 drain 필터 다음에 PDB 사전 검사를 적용하고,
// Pod마다 결과 하나를 입력 순서대로 반환하는지 확인합니다.
func TestAnalyze(t *testing.T) {
	webSelector := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}
	webPDB := newPDB("shop", "web-pdb", webSelector)
	webPDB.Status.DisruptionsAllowed = 1
	otherPDB := newPDB("shop", "other-pdb", webSelector)
	webLabels := withLabels(map[string]string{"app": "web"})
	replicaSet := withController("apps/v1", "ReplicaSet", "web")
	daemonSet := withController("apps/v1", "DaemonSet", "agent")

	pendingDaemonPod := newPod("shop", "agent", daemonSet, withPhase(corev1.PodPending))
	uncontrolledPod := newPod("shop", "debug", webLabels)
	unprotectedPod := newPod("payment", "api-0", replicaSet)
	protectedPod := newPod("shop", "web-1", replicaSet, webLabels)

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
			name:     "Pending DaemonSet Pod는 PDB 생략 검사보다 drain 필터가 우선",
			snapshot: Snapshot{Pods: []corev1.Pod{pendingDaemonPod}},
			want: []Verdict{
				{Namespace: "shop", Name: "agent", Result: Skipped, Reason: "DaemonSet (--ignore-daemonsets)"},
			},
		},
		{
			name: "컨트롤러가 없고 PDB가 두 개여도 drain 필터가 우선",
			snapshot: Snapshot{
				Pods: []corev1.Pod{uncontrolledPod},
				PDBs: []policyv1.PodDisruptionBudget{webPDB, otherPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "debug", Result: NeedsFlag, Reason: "no controller (--force)"},
			},
		},
		{
			name: "ReplicaSet Pod에 매칭되는 PDB가 없으면 퇴거 가능",
			snapshot: Snapshot{
				Pods: []corev1.Pod{unprotectedPod},
				PDBs: []policyv1.PodDisruptionBudget{webPDB},
			},
			want: []Verdict{
				{Namespace: "payment", Name: "api-0", Result: Evictable, Reason: "no matching PDB"},
			},
		},
		{
			name: "PDB 하나에 예산이 있어도 미연결 규칙 때문에 판단 불가",
			snapshot: Snapshot{
				Pods: []corev1.Pod{protectedPod},
				PDBs: []policyv1.PodDisruptionBudget{webPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "web-1", Result: Undetermined,
					PDBs: []string{"web-pdb"}, Reason: "PDB analysis rules are not connected yet"},
			},
		},
		{
			name: "Failed DaemonSet Pod는 drain 필터를 통과하고 PDB 검사를 생략",
			snapshot: Snapshot{
				Pods: []corev1.Pod{newPod("shop", "failed-agent", daemonSet, webLabels, withPhase(corev1.PodFailed))},
				PDBs: []policyv1.PodDisruptionBudget{webPDB, otherPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "failed-agent", Result: Evictable, Reason: "Pod phase is Failed"},
			},
		},
		{
			name: "Succeeded 상태이고 컨트롤러가 없으면 PDB 검사를 생략",
			snapshot: Snapshot{
				Pods: []corev1.Pod{newPod("shop", "report", webLabels, withPhase(corev1.PodSucceeded))},
				PDBs: []policyv1.PodDisruptionBudget{webPDB, otherPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "report", Result: Evictable, Reason: "Pod phase is Succeeded"},
			},
		},
		{
			name: "종료된 mirror Pod도 drain 필터에서 제외",
			snapshot: Snapshot{
				Pods: []corev1.Pod{newPod("shop", "static", withMirrorAnnotation(), withPhase(corev1.PodSucceeded))},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "static", Result: Skipped, Reason: "mirror Pod"},
			},
		},
		{
			name: "컨트롤러가 있고 삭제 중이면 매칭되는 PDB가 두 개여도 검사 생략",
			snapshot: Snapshot{
				Pods: []corev1.Pod{newPod("shop", "deleting", replicaSet, webLabels, withDeletionTimestamp())},
				PDBs: []policyv1.PodDisruptionBudget{webPDB, otherPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "deleting", Result: Evictable, Reason: "Pod is being deleted"},
			},
		},
		{
			name: "Unknown phase는 PDB 검사를 생략하지 않고 하나가 매칭되면 판단 불가",
			snapshot: Snapshot{
				Pods: []corev1.Pod{newPod("shop", "unknown", replicaSet, webLabels, withPhase(corev1.PodUnknown))},
				PDBs: []policyv1.PodDisruptionBudget{webPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "unknown", Result: Undetermined,
					PDBs: []string{"web-pdb"}, Reason: "PDB analysis rules are not connected yet"},
			},
		},
		{
			name: "ReplicaSet Pod에 PDB가 두 개 매칭되면 차단",
			snapshot: Snapshot{
				Pods: []corev1.Pod{protectedPod},
				PDBs: []policyv1.PodDisruptionBudget{webPDB, otherPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "web-1", Result: Blocked,
					PDBs: []string{"web-pdb", "other-pdb"}, Reason: "multiple PDBs match: web-pdb, other-pdb"},
			},
		},
		{
			name: "여러 경로의 결과를 Pod마다 하나씩 입력 순서대로 반환",
			snapshot: Snapshot{
				Pods: []corev1.Pod{pendingDaemonPod, protectedPod, unprotectedPod, uncontrolledPod},
				PDBs: []policyv1.PodDisruptionBudget{webPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "agent", Result: Skipped, Reason: "DaemonSet (--ignore-daemonsets)"},
				{Namespace: "shop", Name: "web-1", Result: Undetermined,
					PDBs: []string{"web-pdb"}, Reason: "PDB analysis rules are not connected yet"},
				{Namespace: "payment", Name: "api-0", Result: Evictable, Reason: "no matching PDB"},
				{Namespace: "shop", Name: "debug", Result: NeedsFlag, Reason: "no controller (--force)"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := Analyze(tt.snapshot)
			// Pod가 없을 때 nil과 빈 슬라이스를 모두 결과 0개로 인정합니다.
			if len(got) != len(tt.want) {
				t.Fatalf("len(Analyze()) = %d, want %d: %#v", len(got), len(tt.want), got)
			}
			for i := range tt.want {
				if !reflect.DeepEqual(got[i], tt.want[i]) {
					t.Errorf("Analyze()[%d] = %#v, want %#v", i, got[i], tt.want[i])
				}
			}
		})
	}
}
