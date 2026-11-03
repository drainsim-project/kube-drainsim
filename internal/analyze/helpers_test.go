package analyze

import (
	"maps"

	corev1 "k8s.io/api/core/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// 이 파일의 보조 함수는 테스트 입력용 Pod와 PDB 구조체를 메모리에서 만듭니다.
// 클러스터에 리소스를 생성하거나 변경하지 않습니다.

// podOption은 newPod가 만드는 테스트용 Pod의 필드를 설정하는 함수 타입입니다.
type podOption func(*corev1.Pod)

// newPod는 지정한 네임스페이스와 이름으로 테스트용 Pod 구조체를 메모리에서 만듭니다.
// 기본 실행 상태는 Running이며 Ready 조건은 없고, opts의 설정 함수를 순서대로 적용합니다.
func newPod(namespace, name string, opts ...podOption) corev1.Pod {
	pod := corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Status:     corev1.PodStatus{Phase: corev1.PodRunning},
	}
	for _, opt := range opts {
		opt(&pod)
	}
	return pod
}

// withPhase는 Pod의 실행 상태를 설정하는 함수를 반환합니다.
// 예를 들어 Succeeded를 지정하면 실행을 마친 Pod를 테스트 입력으로 만듭니다.
func withPhase(phase corev1.PodPhase) podOption {
	return func(pod *corev1.Pod) {
		pod.Status.Phase = phase
	}
}

// withLabels는 Pod에 라벨을 추가하는 설정 함수를 반환합니다.
// PDB의 selector와 비교할 라벨을 가진 Pod를 테스트 입력으로 만듭니다.
func withLabels(labels map[string]string) podOption {
	return func(pod *corev1.Pod) {
		if pod.Labels == nil {
			pod.Labels = make(map[string]string, len(labels))
		}
		maps.Copy(pod.Labels, labels)
	}
}

// withController는 Pod를 관리하는 컨트롤러의 API 버전, 종류와 이름을 소유자 참조에 넣는 설정 함수를 반환합니다.
// Controller를 true로 지정하여 DaemonSet이나 ReplicaSet이 관리하는 Pod를 나타냅니다.
func withController(apiVersion, kind, name string) podOption {
	return func(pod *corev1.Pod) {
		pod.OwnerReferences = append(pod.OwnerReferences, metav1.OwnerReference{
			APIVersion: apiVersion,
			Kind:       kind,
			Name:       name,
			Controller: new(true),
		})
	}
}

// withOwner는 일반 소유자 참조를 추가하는 설정 함수를 반환합니다.
// Controller 표시를 생략하여 소유자는 있지만 관리 컨트롤러로 지정되지 않은 Pod를 나타냅니다.
func withOwner(apiVersion, kind, name string) podOption {
	return func(pod *corev1.Pod) {
		pod.OwnerReferences = append(pod.OwnerReferences, metav1.OwnerReference{
			APIVersion: apiVersion,
			Kind:       kind,
			Name:       name,
		})
	}
}

// withMirrorAnnotation은 static Pod를 API 서버에 나타내는 mirror Pod 표시를 붙이는 설정 함수를 반환합니다.
// mirror Pod 여부를 구분할 수 있도록 테스트용 Pod의 annotation을 설정합니다.
func withMirrorAnnotation() podOption {
	return func(pod *corev1.Pod) {
		if pod.Annotations == nil {
			pod.Annotations = map[string]string{}
		}
		pod.Annotations[corev1.MirrorPodAnnotationKey] = "8f14e45fceea167a"
	}
}

// withDeletionTimestamp는 DeletionTimestamp에 고정 시각을 넣는 설정 함수를 반환합니다.
// 삭제 중인 Pod를 테스트 입력으로 만듭니다.
func withDeletionTimestamp() podOption {
	return func(pod *corev1.Pod) {
		deletedAt := metav1.Unix(0, 0)
		pod.DeletionTimestamp = &deletedAt
	}
}

// newPDB는 지정한 네임스페이스와 이름으로 테스트용 PDB 구조체를 메모리에서 만듭니다.
// selector는 적용 대상 Pod를 라벨로 선택하는 조건이며, nil이나 빈 값, 잘못된 조건도 그대로 넣습니다.
func newPDB(namespace, name string, selector *metav1.LabelSelector) policyv1.PodDisruptionBudget {
	return policyv1.PodDisruptionBudget{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name},
		Spec:       policyv1.PodDisruptionBudgetSpec{Selector: selector},
	}
}
