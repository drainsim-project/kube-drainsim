package analyze

import (
	"reflect"
	"testing"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// TestCheckBudget은 원래 예산이 0일 때의 분류와, 같은 PDB를 공유하는 Pod가 남은 예산을 나눠 쓰는지 확인합니다.
// pods[i]는 pdbs[i]에 매칭된 Pod이며, 사례마다 사용 기록 하나를 공유하며 순서대로 판단합니다.
func TestCheckBudget(t *testing.T) {
	// budget은 원래 예산과 정상 Pod 기준값을 지정한 테스트용 PDB를 만듭니다.
	budget := func(namespace, name string, allowed, desired, expected int32) policyv1.PodDisruptionBudget {
		pdb := newPDB(namespace, name, &metav1.LabelSelector{})
		pdb.Status.DisruptionsAllowed = allowed
		pdb.Status.DesiredHealthy = desired
		pdb.Status.ExpectedPods = expected
		return pdb
	}
	zeroPDB := budget("shop", "web-pdb", 0, 2, 3)
	onePDB := budget("shop", "web-pdb", 1, 2, 3)
	twoPDB := budget("shop", "web-pdb", 2, 1, 3)
	apiPDB := budget("shop", "api-pdb", 1, 2, 3)

	tests := []struct {
		name string
		pods []corev1.Pod
		pdbs []policyv1.PodDisruptionBudget
		want []Verdict
	}{
		{
			name: "원래 예산이 0이고 desiredHealthy와 expectedPods가 같으면 차단",
			pods: []corev1.Pod{newPod("shop", "web-1")},
			pdbs: []policyv1.PodDisruptionBudget{budget("shop", "web-pdb", 0, 1, 1)},
			want: []Verdict{
				{Namespace: "shop", Name: "web-1", Result: Blocked, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=0, desiredHealthy=1 >= expectedPods=1"},
			},
		},
		{
			name: "원래 예산이 0이고 desiredHealthy가 expectedPods보다 크면 차단",
			pods: []corev1.Pod{newPod("shop", "web-1")},
			pdbs: []policyv1.PodDisruptionBudget{budget("shop", "web-pdb", 0, 3, 2)},
			want: []Verdict{
				{Namespace: "shop", Name: "web-1", Result: Blocked, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=0, desiredHealthy=3 >= expectedPods=2"},
			},
		},
		{
			// 컨트롤러는 expectedPods가 0이면 예산을 0으로 두며, 이슈의 규칙대로 차단으로 분류합니다.
			name: "원래 예산이 0이고 expectedPods도 0이면 차단",
			pods: []corev1.Pod{newPod("shop", "web-1")},
			pdbs: []policyv1.PodDisruptionBudget{budget("shop", "web-pdb", 0, 0, 0)},
			want: []Verdict{
				{Namespace: "shop", Name: "web-1", Result: Blocked, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=0, desiredHealthy=0 >= expectedPods=0"},
			},
		},
		{
			name: "원래 예산이 0이고 desiredHealthy가 expectedPods보다 작으면 예산 부족",
			pods: []corev1.Pod{newPod("shop", "web-1")},
			pdbs: []policyv1.PodDisruptionBudget{zeroPDB},
			want: []Verdict{
				{Namespace: "shop", Name: "web-1", Result: NoBudget, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=0, desiredHealthy=2 < expectedPods=3"},
			},
		},
		{
			name: "원래 예산이 0이면 같은 PDB의 Pod가 여러 개여도 예산 소진과 구분",
			pods: []corev1.Pod{newPod("shop", "web-1"), newPod("shop", "web-2")},
			pdbs: []policyv1.PodDisruptionBudget{zeroPDB, zeroPDB},
			want: []Verdict{
				{Namespace: "shop", Name: "web-1", Result: NoBudget, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=0, desiredHealthy=2 < expectedPods=3"},
				{Namespace: "shop", Name: "web-2", Result: NoBudget, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=0, desiredHealthy=2 < expectedPods=3"},
			},
		},
		{
			name: "예산 1을 공유하는 Pod 2개는 먼저 판단한 Pod만 퇴거 가능",
			pods: []corev1.Pod{newPod("shop", "web-1"), newPod("shop", "web-2")},
			pdbs: []policyv1.PodDisruptionBudget{onePDB, onePDB},
			want: []Verdict{
				{Namespace: "shop", Name: "web-1", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1"},
				{Namespace: "shop", Name: "web-2", Result: NoBudget, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1, budget used by web-1"},
			},
		},
		{
			name: "예산 1을 공유하는 Pod 3개는 예산을 쓴 Pod만 근거로 기록",
			pods: []corev1.Pod{newPod("shop", "web-1"), newPod("shop", "web-2"), newPod("shop", "web-3")},
			pdbs: []policyv1.PodDisruptionBudget{onePDB, onePDB, onePDB},
			want: []Verdict{
				{Namespace: "shop", Name: "web-1", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1"},
				{Namespace: "shop", Name: "web-2", Result: NoBudget, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1, budget used by web-1"},
				{Namespace: "shop", Name: "web-3", Result: NoBudget, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1, budget used by web-1"},
			},
		},
		{
			name: "예산 2를 공유하는 Pod 3개는 두 개까지 퇴거 가능",
			pods: []corev1.Pod{newPod("shop", "web-1"), newPod("shop", "web-2"), newPod("shop", "web-3")},
			pdbs: []policyv1.PodDisruptionBudget{twoPDB, twoPDB, twoPDB},
			want: []Verdict{
				{Namespace: "shop", Name: "web-1", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=2"},
				{Namespace: "shop", Name: "web-2", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=2"},
				{Namespace: "shop", Name: "web-3", Result: NoBudget, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=2, budget used by web-1, web-2"},
			},
		},
		{
			name: "서로 다른 namespace의 동명 PDB는 예산을 따로 사용",
			pods: []corev1.Pod{newPod("shop", "web-1"), newPod("payment", "web-1")},
			pdbs: []policyv1.PodDisruptionBudget{onePDB, budget("payment", "web-pdb", 1, 2, 3)},
			want: []Verdict{
				{Namespace: "shop", Name: "web-1", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1"},
				{Namespace: "payment", Name: "web-1", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1"},
			},
		},
		{
			name: "같은 namespace의 다른 PDB는 예산을 따로 사용",
			pods: []corev1.Pod{newPod("shop", "web-1"), newPod("shop", "api-1")},
			pdbs: []policyv1.PodDisruptionBudget{onePDB, apiPDB},
			want: []Verdict{
				{Namespace: "shop", Name: "web-1", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1"},
				{Namespace: "shop", Name: "api-1", Result: Evictable, PDBs: []string{"api-pdb"},
					Reason: "PDB api-pdb: disruptionsAllowed=1"},
			},
		},
		{
			name: "다른 PDB의 Pod를 사이에 두고 판단해도 PDB별 사용 기록을 유지",
			pods: []corev1.Pod{newPod("shop", "web-1"), newPod("shop", "api-1"), newPod("shop", "web-2"), newPod("shop", "api-2")},
			pdbs: []policyv1.PodDisruptionBudget{onePDB, apiPDB, onePDB, apiPDB},
			want: []Verdict{
				{Namespace: "shop", Name: "web-1", Result: Evictable, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1"},
				{Namespace: "shop", Name: "api-1", Result: Evictable, PDBs: []string{"api-pdb"},
					Reason: "PDB api-pdb: disruptionsAllowed=1"},
				{Namespace: "shop", Name: "web-2", Result: NoBudget, PDBs: []string{"web-pdb"},
					Reason: "PDB web-pdb: disruptionsAllowed=1, budget used by web-1"},
				{Namespace: "shop", Name: "api-2", Result: NoBudget, PDBs: []string{"api-pdb"},
					Reason: "PDB api-pdb: disruptionsAllowed=1, budget used by api-1"},
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if len(tt.pdbs) != len(tt.pods) || len(tt.want) != len(tt.pods) {
				t.Fatalf("invalid case: %d pods, %d pdbs, %d want", len(tt.pods), len(tt.pdbs), len(tt.want))
			}
			used := budgetUsage{}
			for i, pod := range tt.pods {
				got := checkBudget(pod, tt.pdbs[i], used)
				if !reflect.DeepEqual(got, tt.want[i]) {
					t.Errorf("checkBudget(%s/%s) = %#v, want %#v", pod.Namespace, pod.Name, got, tt.want[i])
				}
			}
		})
	}
}
