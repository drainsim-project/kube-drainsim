package analyze

import (
	"fmt"
	"reflect"
	"slices"
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
	// maxUnavailable 100%에서 Pod 2개 중 1개가 정상인 상태를 전제로 둡니다.
	// desiredHealthy가 0이라 Ready가 아닌 Pod도 정책 예외 없이 예산 판단까지 갑니다.
	webPDB.Status = policyv1.PodDisruptionBudgetStatus{DisruptionsAllowed: 1, CurrentHealthy: 1, ExpectedPods: 2}
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
			name: "PDB 하나에 예산이 남아 있으면 퇴거 가능",
			snapshot: Snapshot{
				Pods: []corev1.Pod{protectedPod},
				PDBs: []policyv1.PodDisruptionBudget{webPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "web-1", Result: Evictable,
					PDBs: []string{"web-pdb"}, Reason: "PDB web-pdb: disruptionsAllowed=1"},
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
			name: "Unknown phase는 PDB 검사를 생략하지 않고 예산 판단까지 진행",
			snapshot: Snapshot{
				Pods: []corev1.Pod{newPod("shop", "unknown", replicaSet, webLabels, withPhase(corev1.PodUnknown))},
				PDBs: []policyv1.PodDisruptionBudget{webPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "unknown", Result: Evictable,
					PDBs: []string{"web-pdb"}, Reason: "PDB web-pdb: disruptionsAllowed=1"},
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
				{Namespace: "shop", Name: "web-1", Result: Evictable,
					PDBs: []string{"web-pdb"}, Reason: "PDB web-pdb: disruptionsAllowed=1"},
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
		{name: "정상 상태의 예산 0은 예산 판단에서 차단으로 분류", ready: corev1.ConditionTrue,
			status:     policyv1.PodDisruptionBudgetStatus{CurrentHealthy: 1, DesiredHealthy: 1, ExpectedPods: 1},
			wantResult: Blocked, wantReason: "PDB web-pdb: disruptionsAllowed=0, desiredHealthy=1 >= expectedPods=1"},
		{name: "Ready Pod의 예산 선차감은 상태 오류가 아닌 예산 부족으로 분류", ready: corev1.ConditionTrue,
			status: policyv1.PodDisruptionBudgetStatus{CurrentHealthy: 3, DesiredHealthy: 2, ExpectedPods: 3}, disrupted: 1,
			wantResult: NoBudget, wantReason: "PDB web-pdb: disruptionsAllowed=0, desiredHealthy=2 < expectedPods=3"},
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

// TestAnalyzeBudget은 상태 검사를 통과한 Pod에 예산 판단을 연결하고, 같은 PDB의 남은 예산을
// Pod의 namespace와 이름순으로 배분하면서 결과는 입력 순서대로 반환하는지 확인합니다.
// 같은 Snapshot을 두 번 분석해 결과가 같고 입력이 바뀌지 않는지도 함께 확인합니다.
func TestAnalyzeBudget(t *testing.T) {
	replicaSet := withController("apps/v1", "ReplicaSet", "web")
	webLabels := withLabels(map[string]string{"app": "web"})
	ready := withReady(corev1.ConditionTrue)
	// newWebPDB는 app=web 라벨을 선택하고 지정한 상태를 가진 PDB를 만듭니다.
	newWebPDB := func(namespace string, status policyv1.PodDisruptionBudgetStatus) policyv1.PodDisruptionBudget {
		pdb := newPDB(namespace, "web-pdb", &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}})
		pdb.Status = status
		return pdb
	}
	// replica 3개, minAvailable 2이고 모두 정상이라 예산이 1인 상태입니다.
	oneBudget := policyv1.PodDisruptionBudgetStatus{DisruptionsAllowed: 1, CurrentHealthy: 3, DesiredHealthy: 2, ExpectedPods: 3}
	stalePDB := newWebPDB("shop", oneBudget)
	stalePDB.Generation = 2
	stalePDB.Status.ObservedGeneration = 1
	frontPDB := newPDB("shop", "front-pdb", &metav1.LabelSelector{MatchLabels: map[string]string{"tier": "front"}})
	frontPDB.Status = oneBudget
	alwaysAllow := policyv1.AlwaysAllow
	alwaysAllowPDB := newWebPDB("shop", oneBudget)
	alwaysAllowPDB.Spec.UnhealthyPodEvictionPolicy = &alwaysAllow

	tests := []struct {
		name     string
		snapshot Snapshot
		want     []Verdict
	}{
		{
			name: "원래 예산 0이고 desiredHealthy가 expectedPods 이상이면 차단",
			snapshot: Snapshot{
				Pods: []corev1.Pod{newPod("shop", "web-a", replicaSet, webLabels, ready)},
				PDBs: []policyv1.PodDisruptionBudget{
					newWebPDB("shop", policyv1.PodDisruptionBudgetStatus{CurrentHealthy: 1, DesiredHealthy: 1, ExpectedPods: 1}),
				},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "web-a", Result: Blocked, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=0, desiredHealthy=1 >= expectedPods=1"},
			},
		},
		{
			name: "원래 예산 0이고 desiredHealthy가 expectedPods 미만이면 예산 부족",
			snapshot: Snapshot{
				Pods: []corev1.Pod{newPod("shop", "web-a", replicaSet, webLabels, ready)},
				PDBs: []policyv1.PodDisruptionBudget{
					newWebPDB("shop", policyv1.PodDisruptionBudgetStatus{CurrentHealthy: 2, DesiredHealthy: 2, ExpectedPods: 3}),
				},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "web-a", Result: NoBudget, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=0, desiredHealthy=2 < expectedPods=3"},
			},
		},
		{
			name: "예산 1을 공유하면 입력 순서와 관계없이 이름순으로 배분하고 결과는 입력 순서",
			snapshot: Snapshot{
				Pods: []corev1.Pod{
					newPod("shop", "web-b", replicaSet, webLabels, ready),
					newPod("shop", "web-a", replicaSet, webLabels, ready),
				},
				PDBs: []policyv1.PodDisruptionBudget{newWebPDB("shop", oneBudget)},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "web-b", Result: NoBudget, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1, budget used by web-a"},
				{Namespace: "shop", Name: "web-a", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1"},
			},
		},
		{
			name: "서로 다른 namespace의 동명 PDB는 예산을 따로 사용",
			snapshot: Snapshot{
				Pods: []corev1.Pod{
					newPod("shop", "web-a", replicaSet, webLabels, ready),
					newPod("payment", "web-a", replicaSet, webLabels, ready),
				},
				PDBs: []policyv1.PodDisruptionBudget{newWebPDB("shop", oneBudget), newWebPDB("payment", oneBudget)},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "web-a", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1"},
				{Namespace: "payment", Name: "web-a", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1"},
			},
		},
		{
			name: "Ready 정책 예외 Pod는 예산을 쓰지 않아 일반 Pod도 퇴거 가능",
			snapshot: Snapshot{
				Pods: []corev1.Pod{
					newPod("shop", "web-a", replicaSet, webLabels),
					newPod("shop", "web-b", replicaSet, webLabels, ready),
				},
				// replica 3개, minAvailable 1이고 web-a만 Ready가 아니라 예산이 1인 상태입니다.
				PDBs: []policyv1.PodDisruptionBudget{newWebPDB("shop", policyv1.PodDisruptionBudgetStatus{
					DisruptionsAllowed: 1, CurrentHealthy: 2, DesiredHealthy: 1, ExpectedPods: 3})},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "web-a", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "Pod is not Ready; PDB web-pdb: unhealthyPodEvictionPolicy=IfHealthyBudget, currentHealthy=2 >= desiredHealthy=1"},
				{Namespace: "shop", Name: "web-b", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1"},
			},
		},
		{
			name: "AlwaysAllow 정책 예외 Pod도 예산을 쓰지 않음",
			snapshot: Snapshot{
				Pods: []corev1.Pod{
					newPod("shop", "web-a", replicaSet, webLabels),
					newPod("shop", "web-b", replicaSet, webLabels, ready),
				},
				PDBs: []policyv1.PodDisruptionBudget{alwaysAllowPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "web-a", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "Pod is not Ready; PDB web-pdb: unhealthyPodEvictionPolicy=AlwaysAllow"},
				{Namespace: "shop", Name: "web-b", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1"},
			},
		},
		{
			name: "PDB 검사를 생략한 Pod는 예산을 쓰지 않음",
			snapshot: Snapshot{
				Pods: []corev1.Pod{
					newPod("shop", "web-a", replicaSet, webLabels, withPhase(corev1.PodPending)),
					newPod("shop", "web-b", replicaSet, webLabels, ready),
					newPod("shop", "web-c", replicaSet, webLabels, ready),
				},
				PDBs: []policyv1.PodDisruptionBudget{newWebPDB("shop", oneBudget)},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "web-a", Result: Evictable, Reason: "Pod phase is Pending"},
				{Namespace: "shop", Name: "web-b", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1"},
				{Namespace: "shop", Name: "web-c", Result: NoBudget, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1, budget used by web-b"},
			},
		},
		{
			name: "PDB 여러 개로 차단된 Pod는 예산을 쓰지 않음",
			snapshot: Snapshot{
				Pods: []corev1.Pod{
					newPod("shop", "web-a", replicaSet, webLabels, withLabels(map[string]string{"tier": "front"}), ready),
					newPod("shop", "web-b", replicaSet, webLabels, ready),
				},
				PDBs: []policyv1.PodDisruptionBudget{newWebPDB("shop", oneBudget), frontPDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "web-a", Result: Blocked, PDBs: []string{"web-pdb", "front-pdb"},
					Reason: "multiple PDBs match: web-pdb, front-pdb"},
				{Namespace: "shop", Name: "web-b", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1"},
			},
		},
		{
			name: "상태 검사에서 판단 불가이면 예산이 있어도 기존 결과 유지",
			snapshot: Snapshot{
				Pods: []corev1.Pod{
					newPod("shop", "web-a", replicaSet, webLabels, ready),
					newPod("shop", "web-b", replicaSet, webLabels, ready),
				},
				PDBs: []policyv1.PodDisruptionBudget{stalePDB},
			},
			want: []Verdict{
				{Namespace: "shop", Name: "web-a", Result: Undetermined, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: status.observedGeneration=1 < metadata.generation=2"},
				{Namespace: "shop", Name: "web-b", Result: Undetermined, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: status.observedGeneration=1 < metadata.generation=2"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			before := copySnapshot(tt.snapshot)
			// 남은 예산을 호출마다 새로 관리하는지 확인하려고 같은 Snapshot을 두 번 분석합니다.
			for call := range 2 {
				if got := Analyze(tt.snapshot); !reflect.DeepEqual(got, tt.want) {
					t.Errorf("Analyze() call %d = %#v, want %#v", call+1, got, tt.want)
				}
			}
			if !reflect.DeepEqual(tt.snapshot, before) {
				t.Error("Analyze() modified the input Snapshot")
			}
		})
	}
}

// copySnapshot은 Analyze가 입력을 바꾸지 않는지 비교하려고 Snapshot의 Pod, PDB, DaemonSet 목록을 깊은 복사합니다.
func copySnapshot(s Snapshot) Snapshot {
	c := Snapshot{NodeName: s.NodeName, Pods: slices.Clone(s.Pods), PDBs: slices.Clone(s.PDBs), DaemonSets: slices.Clone(s.DaemonSets)}
	for i := range c.Pods {
		s.Pods[i].DeepCopyInto(&c.Pods[i])
	}
	for i := range c.PDBs {
		s.PDBs[i].DeepCopyInto(&c.PDBs[i])
	}
	for i := range c.DaemonSets {
		s.DaemonSets[i].DeepCopyInto(&c.DaemonSets[i])
	}
	return c
}
