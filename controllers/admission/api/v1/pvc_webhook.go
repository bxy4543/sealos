package v1

import (
	"context"
	"fmt"

	"k8s.io/api/apps/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kbv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/labring/sealos/controllers/pkg/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var pvcLog = logf.Log.WithName("pvc-validating-webhook")

type PvcValidator struct {
	client.Client
}

func (v *PvcValidator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	ops, isKBOps := obj.(*kbv1alpha1.OpsRequest)
	if isKBOps && ops.Spec.Type == kbv1alpha1.VolumeExpansionType {
		return v.validateKBOpsRequest(ops)
	}
	return nil
}

func (v *PvcValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	oldSts, isSts := oldObj.(*v1beta2.StatefulSet)
	if isSts {
		return v.validateStatefulSet(oldSts, newObj.(*v1beta2.StatefulSet))
	}
	oldOps, isKBOps := oldObj.(*kbv1alpha1.OpsRequest)
	if isKBOps && oldOps.Spec.Type == kbv1alpha1.VolumeExpansionType {
		return v.validateKBOpsRequest(newObj.(*kbv1alpha1.OpsRequest))
	}
	return nil
}

func (v *PvcValidator) validateKBOpsRequest(opsRequest *kbv1alpha1.OpsRequest) error {
	if opsRequest.Spec.VolumeExpansionList == nil {
		return fmt.Errorf("volume expansion list is nil")
	}
	stsName := opsRequest.Spec.ClusterRef + "-" + opsRequest.Spec.VolumeExpansionList[0].ComponentName
	sts := &v1beta2.StatefulSet{}
	if err := v.Client.Get(context.Background(), client.ObjectKey{
		Namespace: opsRequest.Namespace,
		Name:      stsName,
	}, sts); err != nil {
		return fmt.Errorf("failed to get sts: %w", err)
	}
	nodeNames, err := v.getStsPodNodeName(sts)
	if err != nil {
		return fmt.Errorf("failed to get sts pod node name: %w", err)
	}
	err = v.checkStorageCapacity(nodeNames, opsRequest.Spec.VolumeExpansionList[0].VolumeClaimTemplates[0].Storage.Value(), opsRequest.Namespace, opsRequest.Name)
	if err != nil {
		return err
	}
	return nil
}

func (v *PvcValidator) validateStatefulSet(oldSts, newSts *v1beta2.StatefulSet) error {
	podList, err := v.getStsPodNodeName(newSts)
	if err != nil {
		pvcLog.Error(err, "failed to get sts pod node name")
		return nil
	}
	err = v.checkStorageCapacity(podList, newSts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Value(), newSts.Namespace, newSts.Name)
	if err != nil {
		return err
	}
	return nil
}

func (v *PvcValidator) checkStorageCapacity(nodeNames []string, requestedStorage int64, namespace, name string) error {
	for _, nodeName := range nodeNames {
		residualStorage, err := v.newLVMVgTotalFreeQuery(nodeName)
		if err != nil {
			pvcLog.Error(err, "failed to get lvm vgs total free")
			return nil
		}
		scanUpSize := residualStorage - requestedStorage
		if scanUpSize < 0 {
			return fmt.Errorf("pvc %s/%s can not be scaled down", namespace, name)
		}
		if residualStorage < requestedStorage {
			return fmt.Errorf("pvc %s/%s can not be scaled up, residual storage is not enough on node %s: %d left", namespace, name, nodeName, residualStorage)
		}
	}
	return nil
}

func (v *PvcValidator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	return nil
}

func (v *PvcValidator) newLVMVgTotalFreeQuery(node string) (int64, error) {
	prom, err := prometheus.NewPrometheus("")
	if err != nil {
		return 0, err
	}
	residualSize, err := prom.QueryLvmVgsTotalFree(prometheus.QueryParams{
		Node: node,
	})
	if err != nil {
		return 0, fmt.Errorf("failed to query lvm vgs total free: %w", err)
	}
	return int64(residualSize), nil
}

func (v *PvcValidator) getStsPodNodeName(sts *v1beta2.StatefulSet) ([]string, error) {
	podList := &corev1.PodList{}
	err := v.Client.List(context.Background(), podList, client.InNamespace(sts.Namespace), client.MatchingLabels(sts.Spec.Selector.MatchLabels))
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	var nodeNames []string
	for _, pod := range podList.Items {
		nodeNames = append(nodeNames, pod.Spec.NodeName)
	}
	return nodeNames, nil
}
