package kube

import (
	"testing"
)

func TestObjectHasChanges(t *testing.T) {
	t.Fatal("TODO")
	//for _, tt := range []struct {
	//	Lhs, Rhs    corev1.Pod
	//	WantChanges bool
	//}{
	//	{
	//		corev1.Pod{},
	//		corev1.Pod{},
	//		false
	//	},
	//
	//	{
	//		corev1.Pod{
	//			TypeMeta:   metav1.TypeMeta{},
	//			ObjectMeta: metav1.ObjectMeta{},
	//			Spec:       corev1.PodSpec{},
	//			Status:     corev1.PodStatus{},
	//		},
	//		corev1.Pod{
	//			TypeMeta:   metav1.TypeMeta{},
	//			ObjectMeta: metav1.ObjectMeta{},
	//			Spec:       corev1.PodSpec{},
	//			Status:     corev1.PodStatus{},
	//		},
	//		false
	//	},
	//} {
	//	require.Equal(t, tt.WantChanges, fullnode.PodHasChanges())
	//}
}
