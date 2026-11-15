package analyze

// reasonPDBNotAnalyzed는 PDB 판단 규칙이 아직 연결되지 않았음을 나타냅니다.
const reasonPDBNotAnalyzed = "PDB analysis rules are not connected yet"

// Analyze는 s.Pods의 Pod마다 eviction 가능 여부와 근거를 담을 Verdict를 입력 순서대로 반환합니다.
//
// drain 필터와 PDB 사전 검사를 순서대로 적용합니다. PDB가 하나 매칭되어 추가 판단이
// 필요한 Pod는 PDB 정책, 상태 및 예산 검사가 연결될 때까지 Undetermined로 반환합니다.
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
			v = Verdict{
				Namespace: pod.Namespace,
				Name:      pod.Name,
				Result:    Undetermined,
				PDBs:      []string{pdb.Name},
				Reason:    reasonPDBNotAnalyzed,
			}
		}
		verdicts = append(verdicts, v)
	}
	return verdicts
}
