package analyze

import (
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func filterPod(pod corev1.Pod, _ Snapshot) (v Verdict, done bool) {
	controller := metav1.GetControllerOf(&pod)
	finished := pod.Status.Phase == corev1.PodSucceeded || pod.Status.Phase == corev1.PodFailed
	v.Namespace = pod.Namespace
	v.Name = pod.Name

	// kubectl처럼 DaemonSet, mirror Pod, 컨트롤러 없음 순서로 검사
	if !finished && controller != nil &&
		controller.Kind == "DaemonSet" && controller.APIVersion == appsv1.SchemeGroupVersion.String() {
		v.Result = Skipped
		v.Reason = "DaemonSet (--ignore-daemonsets)"
		return v, true
	}
	if _, mirror := pod.Annotations[corev1.MirrorPodAnnotationKey]; mirror {
		v.Result = Skipped
		v.Reason = "mirror Pod"
		return v, true
	}
	if !finished && controller == nil {
		v.Result = NeedsFlag
		v.Reason = "no controller (--force)"
		return v, true
	}
	return Verdict{}, false
}
