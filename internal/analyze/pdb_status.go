package analyze

import (
	"fmt"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
)

// checkPDBStatus는 판단 근거가 부족한 PDB 상태를 확인합니다.
// 상태 검사를 통과하면 결과를 확정하지 않고 예산 판단으로 넘깁니다.
func checkPDBStatus(pod corev1.Pod, pdb policyv1.PodDisruptionBudget) (v Verdict, done bool) {
	var reason string
	switch {
	case pdb.Status.ObservedGeneration < pdb.Generation:
		reason = fmt.Sprintf("status.observedGeneration=%d < metadata.generation=%d", pdb.Status.ObservedGeneration, pdb.Generation)
	case pdb.Status.DisruptionsAllowed < 0:
		reason = fmt.Sprintf("status.disruptionsAllowed=%d < 0", pdb.Status.DisruptionsAllowed)
	case len(pdb.Status.DisruptedPods) > 2000:
		reason = fmt.Sprintf("status.disruptedPods count=%d > 2000", len(pdb.Status.DisruptedPods))
	default:
		for _, condition := range pdb.Status.Conditions {
			if condition.Type == policyv1.DisruptionAllowedCondition && condition.Reason == policyv1.SyncFailedReason {
				reason = "status.conditions[type=DisruptionAllowed].reason=SyncFailed"
				break
			}
		}
	}
	if reason == "" {
		return Verdict{}, false
	}
	return Verdict{
		Namespace: pod.Namespace,
		Name:      pod.Name,
		Result:    Undetermined,
		PDBs:      []string{pdb.Name},
		Reason:    "PDB " + pdb.Name + ": " + reason,
	}, true
}
