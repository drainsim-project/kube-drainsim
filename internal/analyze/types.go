// Package analyze는 노드를 비우는 kubectl drain을 실행하기 전에, 각 Pod를 퇴거할 수 있는지와
// PDB나 Pod의 관리 방식 때문에 eviction이 거절되거나 생략되는 이유를 분석합니다.
//
// 판단 규칙과 결과 분류는 docs/design.md를 따릅니다.
package analyze

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
)

// Result는 Pod 하나에 대해 퇴거 허용, PDB로 인한 퇴거 거절, drain 옵션 필요,
// eviction 대상 제외 또는 판단 불가 중 어느 상태인지 나타내는 분류 값입니다.
type Result string

const (
	// Evictable은 분석 시점에 PDB 조건상 Pod의 eviction이 허용되는 상태입니다.
	Evictable Result = "evictable"
	// NoBudget은 PDB의 허용 eviction 수가 부족해 지금 이 Pod를 퇴거할 수 없는 상태입니다.
	NoBudget Result = "no-budget"
	// Blocked는 PDB 설정 때문에 Pod eviction이 거절되는 상태입니다. 예상한 Pod가 모두 정상이어도
	// 허용 eviction 수가 0이거나, Pod에 PDB가 2개 이상 적용되는 경우입니다.
	Blocked Result = "blocked"
	// NeedsFlag는 필요한 kubectl drain 옵션이 빠져 Pod를 eviction하기 전에 명령이 중단되는 상태입니다.
	// 예를 들어 실행 중이며 관리 컨트롤러가 없는 Pod를 퇴거하려면 --force가 필요합니다.
	NeedsFlag Result = "needs-flag"
	// Skipped는 kubectl drain이 Pod를 eviction 대상에서 제외하는 상태입니다.
	// 예: --ignore-daemonsets가 지정된 상태에서 실행 중인 DaemonSet Pod입니다.
	Skipped Result = "skipped"
	// Unknown은 Pod의 eviction 가능 여부를 판단할 수 없는 상태입니다.
	Unknown Result = "unknown"
)

// Snapshot은 노드 하나의 Pod eviction 가능 여부를 분석하기 위한 입력 데이터입니다.
// 대상 노드 이름과 조회한 Pod, PDB 및 DaemonSet 목록을 묶어서 Analyze에 전달합니다.
type Snapshot struct {
	// NodeName은 drain 전에 Pod eviction 가능 여부를 분석할 노드의 이름입니다.
	NodeName string
	// Pods는 대상 노드에 배치된 Pod 목록입니다. Analyze는 목록 순서대로 Pod별 결과를 반환합니다.
	Pods []corev1.Pod
	// PDBs는 각 Pod에 적용되는 PDB를 찾기 위한 후보 목록입니다.
	// 다른 네임스페이스의 PDB도 포함할 수 있으므로, Pod와 같은 네임스페이스에서 selector와 라벨을 비교합니다.
	PDBs []policyv1.PodDisruptionBudget
	// DaemonSets는 Pod의 소유자 참조가 가리키는 DaemonSet이 아직 존재하는지 확인하기 위한 목록입니다.
	DaemonSets []appsv1.DaemonSet
}

// Verdict는 노드 drain 때 Pod 하나를 eviction할 수 있는지와 그 이유를 담은 분석 결과입니다.
type Verdict struct {
	// Namespace는 분석한 Pod가 속한 네임스페이스입니다.
	Namespace string
	// Name은 분석한 Pod의 이름입니다. Namespace와 함께 어느 Pod의 결과인지 식별합니다.
	Name string
	// Result는 이 Pod의 eviction 가능 여부와 drain 처리 방식을 나타내는 분류 값입니다.
	Result Result
	// PDBs는 이 Pod의 eviction 허용 여부를 판단할 때 확인한 PDB 이름 목록입니다.
	// 적용되는 PDB가 없으면 비어 있고, 2개 이상 적용되면 해당 이름을 모두 담습니다.
	PDBs []string
	// Reason은 Result가 나온 원인을 사용자에게 설명하는 문구입니다.
	// 같은 Skipped라도 "mirror Pod"와 "DaemonSet (--ignore-daemonsets)"처럼 원인이 다르므로,
	// Result만으로 알 수 없는 제외 조건이나 PDB 상태 값을 기록합니다.
	Reason string
}
