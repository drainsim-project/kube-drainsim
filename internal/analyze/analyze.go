package analyze

// reasonBudgetNotAnalyzed는 예산 판단이 아직 연결되지 않았음을 나타냅니다.
const reasonBudgetNotAnalyzed = "PDB budget analysis is not connected yet"

// Analyze는 s.Pods의 Pod마다 eviction 가능 여부와 근거를 담을 Verdict를 입력 순서대로 반환합니다.
//
// drain 필터, Ready 정책 예외를 포함한 PDB 사전 검사, PDB 상태 검사를 순서대로 적용합니다.
// 상태 검사를 통과한 Pod는 예산 판단이 연결될 때까지 Undetermined로 반환합니다.
func Analyze(s Snapshot) []Verdict {
	// 결과 개수는 0으로 시작하고, Pod 수만큼 추가할 공간을 미리 확보합니다.
	verdicts := make([]Verdict, 0, len(s.Pods))
	for _, pod := range s.Pods {
		if v, done := filterPod(pod, s); done {
			verdicts = append(verdicts, v)
			continue
		}

		v, pdb, done := precheckPod(pod, s)
		if !done {
			v, done = checkPDBStatus(pod, *pdb)
		}
		if !done {
			v = Verdict{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				Result:    Undetermined,
				PDBs:      []string{pdb.Name},
				Reason:    reasonBudgetNotAnalyzed,
			}
		}
		verdicts = append(verdicts, v)
	}
	return verdicts
}
