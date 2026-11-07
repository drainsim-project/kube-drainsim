package analyze

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestPrecheckPod는 PDB 검사 생략 조건, namespace와 selector 매칭 및 반환 규칙을 확인합니다.
// 공통 도우미로 입력을 만들고 PDB 정책과 예산은 검사하지 않습니다.
func TestPrecheckPod(t *testing.T) {
	webSelector := &metav1.LabelSelector{MatchLabels: map[string]string{"app": "web"}}
	webPDB := newPDB("shop", "web-pdb", webSelector)
	otherPDB := newPDB("shop", "other-pdb", webSelector)
	thirdPDB := newPDB("shop", "third-pdb", webSelector)
	invalidPDB := newPDB("shop", "invalid-pdb", &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "app", Operator: "Invalid", Values: []string{"web"}},
		},
	})
	emptyPDB := newPDB("shop", "empty-pdb", &metav1.LabelSelector{})
	expressionPDB := newPDB("shop", "expression-pdb", &metav1.LabelSelector{
		MatchExpressions: []metav1.LabelSelectorRequirement{
			{Key: "app", Operator: metav1.LabelSelectorOpIn, Values: []string{"web", "api"}},
		},
	})
	webPod := newPod("shop", "web-1", withLabels(map[string]string{"app": "web"}))
	noPDB := Verdict{Namespace: "shop", Name: "web-1", Result: Evictable, Reason: "no matching PDB"}

	tests := []struct {
		name     string
		pod      corev1.Pod
		pdbs     []policyv1.PodDisruptionBudget
		want     Verdict
		wantPDB  *policyv1.PodDisruptionBudget
		wantDone bool
	}{
		{
			name: "Pending이면 PDB 검사 생략",
			pod: newPod("shop", "web-1", withPhase(corev1.PodPending),
				withLabels(map[string]string{"app": "web"})),
			pdbs:     []policyv1.PodDisruptionBudget{webPDB, otherPDB},
			want:     Verdict{Namespace: "shop", Name: "web-1", Result: Evictable, Reason: "Pod phase is Pending"},
			wantDone: true,
		},
		{
			name: "Succeeded이면 PDB 검사 생략",
			pod:  newPod("batch", "report", withPhase(corev1.PodSucceeded)),
			pdbs: []policyv1.PodDisruptionBudget{
				newPDB("batch", "batch-pdb", &metav1.LabelSelector{}),
			},
			want:     Verdict{Namespace: "batch", Name: "report", Result: Evictable, Reason: "Pod phase is Succeeded"},
			wantDone: true,
		},
		{
			name: "Failed이면 PDB 검사 생략",
			pod: newPod("shop", "web-1", withPhase(corev1.PodFailed),
				withLabels(map[string]string{"app": "web"})),
			pdbs:     []policyv1.PodDisruptionBudget{webPDB},
			want:     Verdict{Namespace: "shop", Name: "web-1", Result: Evictable, Reason: "Pod phase is Failed"},
			wantDone: true,
		},
		{
			name:     "삭제 중이면 PDB 검사 생략",
			pod:      newPod("shop", "web-1", withDeletionTimestamp()),
			want:     Verdict{Namespace: "shop", Name: "web-1", Result: Evictable, Reason: "Pod is being deleted"},
			wantDone: true,
		},
		{
			name: "삭제 중이면 PDB 두 개가 매칭되어도 생략 조건 우선",
			pod: newPod("shop", "web-1", withDeletionTimestamp(),
				withLabels(map[string]string{"app": "web"})),
			pdbs:     []policyv1.PodDisruptionBudget{webPDB, otherPDB},
			want:     Verdict{Namespace: "shop", Name: "web-1", Result: Evictable, Reason: "Pod is being deleted"},
			wantDone: true,
		},
		{
			name:    "Running이고 PDB 하나가 매칭되면 결과를 확정하지 않음",
			pod:     webPod,
			pdbs:    []policyv1.PodDisruptionBudget{webPDB},
			wantPDB: &webPDB,
		},
		{
			name: "Unknown도 생략하지 않고 PDB 매칭 진행",
			pod: newPod("shop", "web-1", withPhase(corev1.PodUnknown),
				withLabels(map[string]string{"app": "web"})),
			pdbs:    []policyv1.PodDisruptionBudget{webPDB},
			wantPDB: &webPDB,
		},
		{
			name:     "PDB 목록이 비어 있으면 퇴거 가능",
			pod:      webPod,
			want:     noPDB,
			wantDone: true,
		},
		{
			name:     "라벨이 일치하지 않으면 퇴거 가능",
			pod:      newPod("shop", "web-1", withLabels(map[string]string{"app": "api"})),
			pdbs:     []policyv1.PodDisruptionBudget{webPDB},
			want:     noPDB,
			wantDone: true,
		},
		{
			name:     "다른 namespace의 PDB는 매칭하지 않음",
			pod:      webPod,
			pdbs:     []policyv1.PodDisruptionBudget{newPDB("payment", "web-pdb", webSelector)},
			want:     noPDB,
			wantDone: true,
		},
		{
			name: "PDB 두 개가 매칭되면 이름과 이유를 모두 기록",
			pod:  webPod,
			pdbs: []policyv1.PodDisruptionBudget{webPDB, otherPDB},
			want: Verdict{Namespace: "shop", Name: "web-1", Result: Blocked,
				PDBs: []string{"web-pdb", "other-pdb"}, Reason: "multiple PDBs match: web-pdb, other-pdb"},
			wantDone: true,
		},
		{
			name: "PDB 세 개가 매칭되어도 모든 이름을 기록",
			pod:  webPod,
			pdbs: []policyv1.PodDisruptionBudget{webPDB, otherPDB, thirdPDB},
			want: Verdict{Namespace: "shop", Name: "web-1", Result: Blocked,
				PDBs: []string{"web-pdb", "other-pdb", "third-pdb"}, Reason: "multiple PDBs match: web-pdb, other-pdb, third-pdb"},
			wantDone: true,
		},
		{
			name:     "nil selector는 매칭하지 않음",
			pod:      webPod,
			pdbs:     []policyv1.PodDisruptionBudget{newPDB("shop", "nil-pdb", nil)},
			want:     noPDB,
			wantDone: true,
		},
		{
			name:    "빈 selector는 같은 namespace의 라벨 없는 Pod에도 매칭",
			pod:     newPod("shop", "web-1"),
			pdbs:    []policyv1.PodDisruptionBudget{emptyPDB},
			wantPDB: &emptyPDB,
		},
		{
			name:     "빈 selector라도 다른 namespace에는 매칭하지 않음",
			pod:      webPod,
			pdbs:     []policyv1.PodDisruptionBudget{newPDB("payment", "empty-pdb", &metav1.LabelSelector{})},
			want:     noPDB,
			wantDone: true,
		},
		{
			name:     "잘못된 selector는 매칭하지 않음",
			pod:      webPod,
			pdbs:     []policyv1.PodDisruptionBudget{invalidPDB},
			want:     noPDB,
			wantDone: true,
		},
		{
			name:    "잘못된 selector를 건너뛰고 나머지 PDB도 검사",
			pod:     webPod,
			pdbs:    []policyv1.PodDisruptionBudget{invalidPDB, webPDB},
			wantPDB: &webPDB,
		},
		{
			name:    "유효한 matchExpressions 조건으로 매칭",
			pod:     webPod,
			pdbs:    []policyv1.PodDisruptionBudget{expressionPDB},
			wantPDB: &expressionPDB,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, pdb, done := precheckPod(tt.pod, Snapshot{PDBs: tt.pdbs})
			if done != tt.wantDone {
				t.Errorf("precheckPod() done = %t, want %t", done, tt.wantDone)
			}
			// 미확정 결과도 비교하여 PDB 하나가 매칭된 경우를 evictable과 구분합니다.
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("precheckPod() verdict = %#v, want %#v", got, tt.want)
			}
			if !reflect.DeepEqual(pdb, tt.wantPDB) {
				t.Errorf("precheckPod() pdb = %#v, want %#v", pdb, tt.wantPDB)
			}
		})
	}
}
