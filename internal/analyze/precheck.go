package analyze

import (
	"fmt"
	"strings"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
)

// precheckPod는 PDB 검사 생략 여부와 적용되는 PDB를 확인합니다.
// PDB가 하나이면 Ready 정책 예외를 확인하고, 해당하지 않으면 다음 검사로 넘깁니다.
func precheckPod(pod corev1.Pod, s Snapshot) (v Verdict, pdb *policyv1.PodDisruptionBudget, done bool) {
	v.Namespace = pod.Namespace
	v.Name = pod.Name

	// eviction API처럼 삭제 중이거나 Pending, Succeeded, Failed이면 PDB 매칭 전에 반환합니다.
	if !pod.DeletionTimestamp.IsZero() {
		v.Result = Evictable
		v.Reason = "Pod is being deleted"
		return v, nil, true
	}
	if pod.Status.Phase == corev1.PodPending || pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed {
		v.Result = Evictable
		v.Reason = "Pod phase is " + string(pod.Status.Phase)
		return v, nil, true
	}

	var matchedPDBNames []string
	for i := range s.PDBs {
		candidate := &s.PDBs[i]
		if candidate.Namespace != pod.Namespace {
			continue
		}
		selector, err := metav1.LabelSelectorAsSelector(candidate.Spec.Selector)
		if err != nil {
			// 잘못된 selector는 eviction API와 동일하게 매칭 대상에서 제외합니다.
			continue
		}
		if selector.Matches(labels.Set(pod.Labels)) {
			matchedPDBNames = append(matchedPDBNames, candidate.Name)
			pdb = candidate
		}
	}

	switch len(matchedPDBNames) {
	case 0:
		v.Result = Evictable
		v.Reason = "no matching PDB"
		return v, nil, true
	case 1:
		for _, condition := range pod.Status.Conditions {
			if condition.Type == corev1.PodReady {
				if condition.Status == corev1.ConditionTrue {
					return Verdict{}, pdb, false
				}
				break
			}
		}
		// Ready가 아닌 Pod의 정책 예외는 PDB 상태와 예산 검사보다 먼저 적용합니다.
		policy := pdb.Spec.UnhealthyPodEvictionPolicy
		if policy != nil && *policy == policyv1.AlwaysAllow {
			v.Result = Evictable
			v.PDBs = matchedPDBNames
			v.Reason = "Pod is not Ready; PDB " + pdb.Name + ": unhealthyPodEvictionPolicy=AlwaysAllow"
			return v, nil, true
		}
		if (policy == nil || *policy == policyv1.IfHealthyBudget) &&
			pdb.Status.DesiredHealthy > 0 && pdb.Status.CurrentHealthy >= pdb.Status.DesiredHealthy {
			v.Result = Evictable
			v.PDBs = matchedPDBNames
			v.Reason = fmt.Sprintf("Pod is not Ready; PDB %s: unhealthyPodEvictionPolicy=IfHealthyBudget, currentHealthy=%d >= desiredHealthy=%d",
				pdb.Name, pdb.Status.CurrentHealthy, pdb.Status.DesiredHealthy)
			return v, nil, true
		}
		return Verdict{}, pdb, false
	default:
		v.Result = Blocked
		v.PDBs = matchedPDBNames
		v.Reason = "multiple PDBs match: " + strings.Join(matchedPDBNames, ", ")
		return v, nil, true
	}
}
