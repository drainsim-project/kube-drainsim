package analyze

import (
	"fmt"
	"reflect"
	"testing"

	appsv1 "k8s.io/api/apps/v1"
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

	owner := appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Namespace: "shop", Name: "agent"}}
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
			name: "Pending DaemonSet Pod는 PDB 생략 검사보다 drain 필터가 우선",
			// 소유자 DaemonSet이 존재하는 Pending Pod
			snapshot: Snapshot{
				Pods:       []corev1.Pod{pendingDaemonPod},
				DaemonSets: []appsv1.DaemonSet{owner},
			},
			// Pending이어도 DaemonSet 필터가 먼저라 evictable이 아닌 skipped로 처리
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
					PDBs: []string{"web-pdb"}, Reason: "PDB budget analysis is not connected yet"},
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
					PDBs: []string{"web-pdb"}, Reason: "PDB budget analysis is not connected yet"},
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
				Pods:       []corev1.Pod{pendingDaemonPod, protectedPod, unprotectedPod, uncontrolledPod},
				DaemonSets: []appsv1.DaemonSet{owner},
				PDBs:       []policyv1.PodDisruptionBudget{webPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "agent", Result: Skipped, Reason: "DaemonSet (--ignore-daemonsets)"},
				{Namespace: "shop", Name: "web-1", Result: Undetermined,
					PDBs: []string{"web-pdb"}, Reason: "PDB budget analysis is not connected yet"},
				{Namespace: "payment", Name: "api-0", Result: Evictable, Reason: "no matching PDB"},
				{Namespace: "shop", Name: "debug", Result: NeedsFlag, Reason: "no controller (--force)"},
			},
		},
		{
			name: "소유자 없는 Running DaemonSet Pod",
			snapshot: Snapshot{
				Pods: []corev1.Pod{newPod("shop", "orphan-agent", daemonSet, webLabels, withPhase(corev1.PodRunning))},
				PDBs: []policyv1.PodDisruptionBudget{webPDB, otherPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "orphan-agent", Result: NeedsFlag, Reason: "DaemonSet shop/agent not found (--force required)"},
			},
		},
		{
			name: "소유자 없는 Pending DaemonSet Pod",
			snapshot: Snapshot{
				Pods: []corev1.Pod{newPod("shop", "orphan-agent", daemonSet, webLabels, withPhase(corev1.PodPending))},
				PDBs: []policyv1.PodDisruptionBudget{webPDB, otherPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "orphan-agent", Result: NeedsFlag, Reason: "DaemonSet shop/agent not found (--force required)"},
			},
		},
		{
			name: "소유자 없는 Succeeded DaemonSet Pod",
			snapshot: Snapshot{
				Pods: []corev1.Pod{newPod("shop", "orphan-agent", daemonSet, webLabels, withPhase(corev1.PodSucceeded))},
				PDBs: []policyv1.PodDisruptionBudget{webPDB, otherPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "orphan-agent", Result: Evictable, Reason: "Pod phase is Succeeded"},
			},
		},
		{
			name: "소유자 없는 Failed DaemonSet Pod",
			snapshot: Snapshot{
				Pods: []corev1.Pod{newPod("shop", "orphan-agent", daemonSet, webLabels, withPhase(corev1.PodFailed))},
				PDBs: []policyv1.PodDisruptionBudget{webPDB, otherPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "orphan-agent", Result: Evictable, Reason: "Pod phase is Failed"},
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

// TestAnalyzePDBStatus는 상태 검사 연결과 Ready 정책 예외의 우선순위를 확인합니다.
func TestAnalyzePDBStatus(t *testing.T) {
	alwaysAllow := policyv1.AlwaysAllow
	ifHealthyBudget := policyv1.IfHealthyBudget
	tests := []struct {
		name       string
		generation int64
		status     policyv1.PodDisruptionBudgetStatus
		disrupted  int
		policy     *policyv1.UnhealthyPodEvictionPolicyType
		ready      corev1.ConditionStatus
		wantResult Result
		wantReason string
	}{
		{name: "갱신 전 세대는 상태 검사로 판단 불가", generation: 2,
			status:     policyv1.PodDisruptionBudgetStatus{ObservedGeneration: 1},
			wantResult: Undetermined, wantReason: "PDB web-pdb: status.observedGeneration=1 < metadata.generation=2"},
		{name: "음수 예산은 상태 검사로 판단 불가",
			status:     policyv1.PodDisruptionBudgetStatus{DisruptionsAllowed: -1},
			wantResult: Undetermined, wantReason: "PDB web-pdb: status.disruptionsAllowed=-1 < 0"},
		{name: "disruptedPods 2001개는 상태 검사로 판단 불가", disrupted: 2001,
			wantResult: Undetermined, wantReason: "PDB web-pdb: status.disruptedPods count=2001 > 2000"},
		{name: "SyncFailed는 상태 검사로 판단 불가",
			status: policyv1.PodDisruptionBudgetStatus{Conditions: []metav1.Condition{
				{Type: policyv1.DisruptionAllowedCondition, Reason: policyv1.SyncFailedReason},
			}},
			wantResult: Undetermined, wantReason: "PDB web-pdb: status.conditions[type=DisruptionAllowed].reason=SyncFailed"},
		{name: "정상 상태의 예산 0은 예산 판단 미연결로 판단 불가",
			wantResult: Undetermined, wantReason: "PDB budget analysis is not connected yet"},
		{name: "Ready Pod의 예산 선차감은 상태 오류로 분류하지 않음", ready: corev1.ConditionTrue,
			status: policyv1.PodDisruptionBudgetStatus{CurrentHealthy: 3, DesiredHealthy: 2}, disrupted: 1,
			wantResult: Undetermined, wantReason: "PDB budget analysis is not connected yet"},
		{name: "AlwaysAllow 예외는 세대 지연과 음수 예산 및 항목 초과보다 우선", generation: 2,
			policy: &alwaysAllow, status: policyv1.PodDisruptionBudgetStatus{DisruptionsAllowed: -1}, disrupted: 2001,
			wantResult: Evictable, wantReason: "Pod is not Ready; PDB web-pdb: unhealthyPodEvictionPolicy=AlwaysAllow"},
		{name: "기본 정책 예외는 SyncFailed보다 우선",
			status: policyv1.PodDisruptionBudgetStatus{CurrentHealthy: 2, DesiredHealthy: 2, Conditions: []metav1.Condition{
				{Type: policyv1.DisruptionAllowedCondition, Reason: policyv1.SyncFailedReason},
			}},
			wantResult: Evictable, wantReason: "Pod is not Ready; PDB web-pdb: unhealthyPodEvictionPolicy=IfHealthyBudget, currentHealthy=2 >= desiredHealthy=2"},
		{name: "IfHealthyBudget 예외는 세대 지연보다 우선", generation: 2, policy: &ifHealthyBudget,
			status:     policyv1.PodDisruptionBudgetStatus{CurrentHealthy: 2, DesiredHealthy: 2},
			wantResult: Evictable, wantReason: "Pod is not Ready; PDB web-pdb: unhealthyPodEvictionPolicy=IfHealthyBudget, currentHealthy=2 >= desiredHealthy=2"},
		{name: "Ready=True이면 AlwaysAllow여도 상태 검사 진행", generation: 2, policy: &alwaysAllow, ready: corev1.ConditionTrue,
			wantResult: Undetermined, wantReason: "PDB web-pdb: status.observedGeneration=0 < metadata.generation=2"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := newPod("shop", "web-1", withController("apps/v1", "ReplicaSet", "web"),
				withLabels(map[string]string{"app": "web"}))
			if tt.ready != "" {
				pod.Status.Conditions = []corev1.PodCondition{{Type: corev1.PodReady, Status: tt.ready}}
			}
			pdb := newPDB("shop", "web-pdb", &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}})
			pdb.Generation = tt.generation
			pdb.Spec.UnhealthyPodEvictionPolicy = tt.policy
			pdb.Status = tt.status
			if tt.disrupted > 0 {
				pdb.Status.DisruptedPods = make(map[string]metav1.Time, tt.disrupted)
				for i := 0; i < tt.disrupted; i++ {
					pdb.Status.DisruptedPods[fmt.Sprintf("web-%d", i)] = metav1.Unix(0, 0)
				}
			}
			got := Analyze(Snapshot{Pods: []corev1.Pod{pod}, PDBs: []policyv1.PodDisruptionBudget{pdb}})
			want := []Verdict{{Namespace: "shop", Name: "web-1", Result: tt.wantResult,
				PDBs: []string{"web-pdb"}, Reason: tt.wantReason}}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("Analyze() = %#v, want %#v", got, want)
			}
		})
	}
}
