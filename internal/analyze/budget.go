package analyze

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	"k8s.io/apimachinery/pkg/types"
)

// budgetUsage는 Analyze 호출 한 번에서 PDB별로 예산을 사용한 Pod 이름을 사용한 순서대로 기록합니다.
// PDB는 namespace와 이름으로 구분하고, 남은 예산은 원래 disruptionsAllowed에서 기록된 Pod 수를 뺀 값입니다.
type budgetUsage map[types.NamespacedName][]string

// checkBudget은 상태 검사를 통과해 disruptionsAllowed가 0 이상인 PDB로 Pod의 예산을 판단합니다.
//
// 원래 예산이 0이면 desiredHealthy와 expectedPods를 비교해 blocked와 no-budget을 구분합니다.
// 원래 예산이 양수이면 남은 예산이 있을 때 evictable로 분류하고 used에 기록해 남은 예산을 1 줄이며,
// 남은 예산이 없으면 먼저 예산을 사용한 Pod를 근거로 no-budget으로 분류합니다. PDB 상태는 변경하지 않습니다.
func checkBudget(pod corev1.Pod, pdb policyv1.PodDisruptionBudget, used budgetUsage) Verdict {
	v := Verdict{Namespace: pod.Namespace, Name: pod.Name, PDBs: []string{pdb.Name}}
	allowed := pdb.Status.DisruptionsAllowed
	if allowed == 0 {
		desired, expected := pdb.Status.DesiredHealthy, pdb.Status.ExpectedPods
		if desired >= expected {
			// 예상한 Pod가 모두 정상이어도 예산이 생기지 않는 설정입니다.
			v.Result = Blocked
			v.Reason = fmt.Sprintf("PDB %s: disruptionsAllowed=0, desiredHealthy=%d >= expectedPods=%d", pdb.Name, desired, expected)
		} else {
			v.Result = NoBudget
			v.Reason = fmt.Sprintf("PDB %s: disruptionsAllowed=0, desiredHealthy=%d < expectedPods=%d", pdb.Name, desired, expected)
		}
		return v
	}

	key := types.NamespacedName{Namespace: pdb.Namespace, Name: pdb.Name}
	if len(used[key]) >= int(allowed) {
		v.Result = NoBudget
		v.Reason = fmt.Sprintf("PDB %s: disruptionsAllowed=%d, budget used by %s", pdb.Name, allowed, strings.Join(used[key], ", "))
		return v
	}
	used[key] = append(used[key], pod.Name)
	v.Result = Evictable
	v.Reason = fmt.Sprintf("PDB %s: disruptionsAllowed=%d", pdb.Name, allowed)
	return v
}
