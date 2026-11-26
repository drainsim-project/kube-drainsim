package analyze

import (
	"cmp"
	"slices"

	policyv1 "k8s.io/api/policy/v1"
)

// budgetRequest는 상태 검사까지 결과가 정해지지 않아 예산 판단을 기다리는 Pod의 입력 위치와 매칭된 PDB입니다.
type budgetRequest struct {
	index int
	pdb   *policyv1.PodDisruptionBudget
}

// Analyze는 s.Pods의 Pod마다 eviction 가능 여부와 근거를 담은 Verdict를 입력 순서대로 반환합니다.
//
// drain 필터, Ready 정책 예외를 포함한 PDB 사전 검사, PDB 상태 검사를 순서대로 적용하고,
// 결과가 정해지지 않은 Pod에 예산 판단을 적용합니다. 같은 PDB를 공유하는 Pod는 남은 예산을 나눠 쓰며,
// 예산은 입력 순서와 관계없이 Pod의 namespace와 이름순으로 배분합니다. 이 순서는 실제 eviction 순서를 보장하지 않습니다.
func Analyze(s Snapshot) []Verdict {
	// 예산 판단은 다른 검사를 모두 마친 뒤 하므로, 결과는 Pod의 입력 위치에 기록합니다.
	verdicts := make([]Verdict, len(s.Pods))
	var requests []budgetRequest
	for i, pod := range s.Pods {
		// drain 필터에서 결과가 정해진 Pod는 예산 배분에 넣지 않습니다. needs-flag Pod도 빠지므로, 안내한 --force 같은
		// 옵션을 붙여 drain하면 그 Pod가 같은 PDB의 예산을 함께 써서 evictable로 판단한 Pod가 거절될 수 있습니다.
		// drain 옵션을 입력으로 받게 되면 옵션 때문에 퇴거될 Pod도 예산 배분에 넣어야 합니다.
		if v, done := filterPod(pod, s); done {
			verdicts[i] = v
			continue
		}

		v, pdb, done := precheckPod(pod, s)
		if !done {
			v, done = checkPDBStatus(pod, *pdb)
		}
		if !done {
			requests = append(requests, budgetRequest{index: i, pdb: pdb})
			continue
		}
		verdicts[i] = v
	}

	slices.SortStableFunc(requests, func(a, b budgetRequest) int {
		podA, podB := &s.Pods[a.index], &s.Pods[b.index]
		return cmp.Or(cmp.Compare(podA.Namespace, podB.Namespace), cmp.Compare(podA.Name, podB.Name))
	})
	used := make(budgetUsage)
	for _, r := range requests {
		verdicts[r.index] = checkBudget(s.Pods[r.index], *r.pdb, used)
	}
	return verdicts
}
