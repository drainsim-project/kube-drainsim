package collect

import (
	"context"
	"errors"
	"reflect"
	"testing"

	"github.com/drainsim-project/kube-drainsim/internal/analyze"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/client-go/kubernetes/fake"
	ktesting "k8s.io/client-go/testing"
)

func daemonPod(namespace, name string, phase corev1.PodPhase) corev1.Pod {
	controller := true
	return corev1.Pod{
		ObjectMeta: metav1.ObjectMeta{Namespace: namespace, Name: name, OwnerReferences: []metav1.OwnerReference{{APIVersion: "apps/v1", Kind: "DaemonSet", Name: "agent", Controller: &controller}}},
		Spec:       corev1.PodSpec{NodeName: "worker-1"},
		Status:     corev1.PodStatus{Phase: phase},
	}
}

func TestPods(t *testing.T) {
	plain := daemonPod("ops", "plain", corev1.PodRunning)
	plain.OwnerReferences = nil
	nonController := daemonPod("ops", "non-controller", corev1.PodRunning)
	nonController.OwnerReferences[0].Controller = nil
	replica := daemonPod("ops", "replica", corev1.PodRunning)
	replica.OwnerReferences[0].Kind = "ReplicaSet"
	oldAPI := daemonPod("ops", "old-api", corev1.PodRunning)
	oldAPI.OwnerReferences[0].APIVersion = "extensions/v1beta1"
	tests := []struct {
		name       string
		pods       []corev1.Pod
		namespaces []string
	}{
		{name: "empty"},
		{name: "lookup and deduplicate by namespace and name", pods: []corev1.Pod{
			daemonPod("ops", "running", corev1.PodRunning),
			daemonPod("ops", "pending", corev1.PodPending),
			daemonPod("other", "unknown", corev1.PodUnknown),
		}, namespaces: []string{"ops", "other"}},
		{name: "retain pods that do not require lookup", pods: []corev1.Pod{
			daemonPod("ops", "succeeded", corev1.PodSucceeded),
			daemonPod("ops", "failed", corev1.PodFailed), plain, nonController, replica, oldAPI,
		}},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset()
			// Inspect the request explicitly; fake clients do not replace API-server filtering tests.
			client.PrependReactor("list", "pods", func(action ktesting.Action) (bool, runtime.Object, error) {
				list := action.(ktesting.ListAction)
				if action.GetNamespace() != metav1.NamespaceAll || list.GetListRestrictions().Fields.String() != "spec.nodeName=worker-1" {
					t.Fatalf("unexpected list request: %#v", action)
				}
				return true, &corev1.PodList{Items: tt.pods}, nil
			})
			var gotNamespaces []string
			var wantDS []appsv1.DaemonSet
			client.PrependReactor("get", "daemonsets", func(action ktesting.Action) (bool, runtime.Object, error) {
				if action.(ktesting.GetAction).GetName() != "agent" {
					t.Fatalf("unexpected owner name: %#v", action)
				}
				gotNamespaces = append(gotNamespaces, action.GetNamespace())
				ds := appsv1.DaemonSet{ObjectMeta: metav1.ObjectMeta{Namespace: action.GetNamespace(), Name: "agent"}}
				wantDS = append(wantDS, ds)
				return true, &ds, nil
			})
			got, err := Pods(context.Background(), client, "worker-1")
			if err != nil {
				t.Fatal(err)
			}
			want := analyze.Snapshot{NodeName: "worker-1", Pods: tt.pods, DaemonSets: wantDS}
			if !reflect.DeepEqual(got, want) {
				t.Fatalf("got %#v, want %#v", got, want)
			}
			if !reflect.DeepEqual(gotNamespaces, tt.namespaces) {
				t.Fatalf("lookup namespaces: %v, want %v", gotNamespaces, tt.namespaces)
			}
			if len(client.Actions()) != 1+len(tt.namespaces) {
				t.Fatalf("unexpected API actions: %#v", client.Actions())
			}
		})
	}
}

func TestPodsErrors(t *testing.T) {
	resource := schema.GroupResource{Group: "apps", Resource: "daemonsets"}
	tests := []struct {
		name    string
		listErr error
		getErr  error
		wantErr bool
	}{
		{name: "missing owner", getErr: apierrors.NewNotFound(resource, "agent")},
		{name: "forbidden owner", getErr: apierrors.NewForbidden(resource, "agent", errors.New("denied")), wantErr: true},
		{name: "owner transport error", getErr: errors.New("connection failed"), wantErr: true},
		{name: "pod list failure", listErr: errors.New("list failed"), wantErr: true},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			client := fake.NewClientset()
			pods := []corev1.Pod{daemonPod("ops", "one", corev1.PodRunning), daemonPod("ops", "two", corev1.PodPending)}
			client.PrependReactor("list", "pods", func(ktesting.Action) (bool, runtime.Object, error) {
				return true, &corev1.PodList{Items: pods}, tt.listErr
			})
			client.PrependReactor("get", "daemonsets", func(ktesting.Action) (bool, runtime.Object, error) {
				return true, nil, tt.getErr
			})
			got, err := Pods(context.Background(), client, "worker-1")
			if tt.wantErr {
				cause := tt.getErr
				if tt.listErr != nil {
					cause = tt.listErr
				}
				if !errors.Is(err, cause) {
					t.Fatalf("got error %v, want wrapped %v", err, cause)
				}
				if !reflect.DeepEqual(got, analyze.Snapshot{}) {
					t.Fatalf("returned partial snapshot: %#v", got)
				}
			} else {
				if err != nil {
					t.Fatal(err)
				}
				if !reflect.DeepEqual(got, analyze.Snapshot{NodeName: "worker-1", Pods: pods}) {
					t.Fatalf("missing owner must retain pods: %#v", got)
				}
			}
			wantActions := 2
			if tt.listErr != nil {
				wantActions = 1
			}
			if len(client.Actions()) != wantActions {
				t.Fatalf("got %d actions, want %d", len(client.Actions()), wantActions)
			}
		})
	}
}

func TestPodsEmptyNode(t *testing.T) {
	client := fake.NewClientset()
	_, err := Pods(context.Background(), client, "")
	if err == nil || len(client.Actions()) != 0 {
		t.Fatalf("empty node must fail before API requests: %v", err)
	}
}
