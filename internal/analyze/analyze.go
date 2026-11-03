package analyze

// reasonNotAnalyzed는 Pod eviction 검사 로직이 아직 연결되지 않았음을 나타내는 설명 문구입니다.
// 로직이 구현되면 이 상수는 삭제합니다.
const reasonNotAnalyzed = "not analyzed yet"

// Analyze는 s.Pods의 Pod마다 eviction 가능 여부와 근거를 담을 Verdict를 입력 순서대로 반환합니다.
//
// 현재는 eviction 검사 로직이 없으므로 Pod의 네임스페이스와 이름만 복사하고,
// Result는 Unknown, Reason은 "not analyzed yet"으로 할당합니다.
func Analyze(s Snapshot) []Verdict {
	// 결과 개수는 0으로 시작하고, Pod 수만큼 추가할 공간을 미리 확보합니다.
	verdicts := make([]Verdict, 0, len(s.Pods))
	for _, pod := range s.Pods {
		verdicts = append(verdicts, Verdict{
			Namespace: pod.Namespace,
			Name:      pod.Name,
			Result:    Unknown,
			Reason:    reasonNotAnalyzed,
		})
	}
	return verdicts
}
