package analyze

import (
	"fmt"
	"reflect"
	"testing"

	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestCheckPDBStatus는 상태 이상과 경계값을 검사하고 정상 상태에서는 판단을 보류하는지 확인합니다.
func TestCheckPDBStatus(t *testing.T) {
	tests := []struct {
		name       string
		generation int64
		status     policyv1.PodDisruptionBudgetStatus
		disrupted  int
		wantReason string
	}{
		{name: "관찰한 세대가 이전이면 판단 불가", generation: 2,
			status:     policyv1.PodDisruptionBudgetStatus{ObservedGeneration: 1},
			wantReason: "PDB web-pdb: status.observedGeneration=1 < metadata.generation=2"},
		{name: "음수 예산이면 판단 불가",
			status:     policyv1.PodDisruptionBudgetStatus{DisruptionsAllowed: -1},
			wantReason: "PDB web-pdb: status.disruptionsAllowed=-1 < 0"},
		{name: "disruptedPods 2000개는 통과", disrupted: 2000},
		{name: "disruptedPods 2001개는 판단 불가", disrupted: 2001,
			wantReason: "PDB web-pdb: status.disruptedPods count=2001 > 2000"},
		{name: "DisruptionAllowed의 SyncFailed는 판단 불가",
			status: policyv1.PodDisruptionBudgetStatus{Conditions: []metav1.Condition{
				{Type: policyv1.DisruptionAllowedCondition, Status: metav1.ConditionFalse, Reason: policyv1.SyncFailedReason},
			}},
			wantReason: "PDB web-pdb: status.conditions[type=DisruptionAllowed].reason=SyncFailed"},
		{name: "다른 조건의 SyncFailed는 해당하지 않음",
			status: policyv1.PodDisruptionBudgetStatus{Conditions: []metav1.Condition{
				{Type: "Other", Reason: policyv1.SyncFailedReason},
			}}},
		{name: "정상 DisruptionAllowed 조건은 통과",
			status: policyv1.PodDisruptionBudgetStatus{Conditions: []metav1.Condition{
				{Type: policyv1.DisruptionAllowedCondition, Status: metav1.ConditionTrue, Reason: "SufficientPods"},
			}}},
		{name: "세대가 같고 예산 0이면 판단 보류", generation: 2,
			status: policyv1.PodDisruptionBudgetStatus{ObservedGeneration: 2}},
		{name: "관찰한 세대가 크면 세대 지연으로 분류하지 않음", generation: 2,
			status: policyv1.PodDisruptionBudgetStatus{ObservedGeneration: 3, DisruptionsAllowed: 1}},
		{name: "예산 선차감으로 정상 수 차이와 달라도 통과",
			status: policyv1.PodDisruptionBudgetStatus{CurrentHealthy: 3, DesiredHealthy: 2, DisruptionsAllowed: 0}, disrupted: 1},
		{name: "여러 이상이 겹치면 세대 검사 우선", generation: 2,
			status: policyv1.PodDisruptionBudgetStatus{ObservedGeneration: 1, DisruptionsAllowed: -1}, disrupted: 2001,
			wantReason: "PDB web-pdb: status.observedGeneration=1 < metadata.generation=2"},
		{name: "음수 예산은 disruptedPods 제한보다 먼저 검사",
			status: policyv1.PodDisruptionBudgetStatus{DisruptionsAllowed: -1}, disrupted: 2001,
			wantReason: "PDB web-pdb: status.disruptionsAllowed=-1 < 0"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			pod := newPod("shop", "web-1")
			pdb := newPDB("shop", "web-pdb", &metav1.LabelSelector{})
			pdb.Generation = tt.generation
			pdb.Status = tt.status
			if tt.disrupted > 0 {
				pdb.Status.DisruptedPods = make(map[string]metav1.Time, tt.disrupted)
				for i := 0; i < tt.disrupted; i++ {
					pdb.Status.DisruptedPods[fmt.Sprintf("web-%d", i)] = metav1.Unix(0, 0)
				}
			}
			want := Verdict{}
			wantDone := tt.wantReason != ""
			if wantDone {
				want = Verdict{Namespace: "shop", Name: "web-1", Result: Undetermined,
					PDBs: []string{"web-pdb"}, Reason: tt.wantReason}
			}
			got, done := checkPDBStatus(pod, pdb)
			if done != wantDone {
				t.Errorf("checkPDBStatus() done = %t, want %t", done, wantDone)
			}
			if !reflect.DeepEqual(got, want) {
				t.Errorf("checkPDBStatus() = %#v, want %#v", got, want)
			}
		})
	}
}
