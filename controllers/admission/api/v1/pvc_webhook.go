package v1

import (
	"context"
	"fmt"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

type PvcValidator struct {
}

func (v *PvcValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (v *PvcValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	//TODO pvc 扩容大于5G 则拦截请求, 禁止缩容
	oldPvc := oldObj.(*corev1.PersistentVolumeClaim)
	newPvc := newObj.(*corev1.PersistentVolumeClaim)
	if oldPvc.Spec.Resources.Requests.Storage().Value() > newPvc.Spec.Resources.Requests.Storage().Value() {
		return fmt.Errorf("pvc %s/%s can not be scaled down", newPvc.Namespace, newPvc.Name)
	}

	return nil
}

func (v *PvcValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}
