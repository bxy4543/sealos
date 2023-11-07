package v1

import (
	"context"
	"fmt"

	admissionv1 "k8s.io/api/admission/v1"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"k8s.io/api/apps/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kbv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	"github.com/labring/sealos/controllers/pkg/prometheus"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var pvcLog = logf.Log.WithName("pvc-validating-webhook")

// +kubebuilder:webhook:path=/validate-opsrequest-sts-pvc,mutating=false,failurePolicy=fail,groups=apps.kubeblocks.io;apps,resources=opsrequests;statefulsets,verbs=create;update;delete,versions=v1alpha1;v1,name=vresources.kb.io,sideEffects=None,admissionReviewVersions={v1,v1beta1}

type PvcValidator struct {
	client.Client
	PromoURL string
}

func (v *PvcValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	var obj = req.Object.Object
	var oldObj = req.OldObject.Object
	var err error
	switch req.Operation {
	case admissionv1.Create:
		err = v.ValidateCreate(ctx, obj)
	case admissionv1.Update:
		err = v.ValidateUpdate(ctx, oldObj, obj)
	}

	if err != nil {
		return admission.Denied(err.Error())
	}

	return admission.Allowed("allowed to commit the request")
}

func (v *PvcValidator) ValidateCreate(_ context.Context, obj runtime.Object) error {
	ops, isKBOps := obj.(*kbv1alpha1.OpsRequest)
	if isKBOps && ops.Spec.Type == kbv1alpha1.VolumeExpansionType {
		return v.validateKBOpsRequest(ops)
	}
	return nil
}

func (v *PvcValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.Object) error {
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

	//app.kubernetes.io/instance=test-name,app.kubernetes.io/managed-by=kubeblocks
	nodeNames, err := v.getPodNodeName(opsRequest.Namespace, client.MatchingLabels{"app.kubernetes.io/instance": opsRequest.Spec.ClusterRef, "app.kubernetes.io/managed-by": "kubeblocks"})
	if err != nil {
		return fmt.Errorf("failed to get sts pod node name: %w", err)
	}
	expansionSize, err := v.getResizeStorageWithOpsRequest(opsRequest)
	if err != nil {
		return fmt.Errorf("failed to get storage with ops request: %w", err)
	}

	err = v.checkStorageCapacity(nodeNames, expansionSize, opsRequest.Namespace, opsRequest.Name)
	if err != nil {
		return err
	}
	return nil
}

func (v *PvcValidator) validateStatefulSet(oldSts, newSts *v1beta2.StatefulSet) error {
	podList, err := v.getPodNodeName(newSts.Namespace, newSts.Spec.Selector.MatchLabels)
	if err != nil {
		pvcLog.Error(err, "failed to get sts pod node name")
		return nil
	}
	err = v.checkStorageCapacity(podList,
		newSts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Value()-oldSts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Value(),
		newSts.Namespace, newSts.Name)
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
		if residualStorage < requestedStorage {
			pvcLog.Error(fmt.Errorf("failed to scaled down pvc"), "pvc can not be scaled up", "namespace", namespace, "pvc name", name, "nodeName", nodeName, "residualStorage", residualStorage, "requestedStorage", requestedStorage)
			return fmt.Errorf("pvc %s/%s can not be scaled down", namespace, name)
		}
	}
	return nil
}

func (v *PvcValidator) newLVMVgTotalFreeQuery(node string) (int64, error) {
	prom, err := prometheus.NewPrometheus(v.PromoURL)
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

func (v *PvcValidator) getPodNodeName(namespace string, matchLabels client.MatchingLabels) ([]string, error) {
	podList := &corev1.PodList{}
	err := v.Client.List(context.Background(), podList, client.InNamespace(namespace), matchLabels)
	if err != nil {
		return nil, fmt.Errorf("failed to list pods: %w", err)
	}
	var nodeNames []string
	for _, pod := range podList.Items {
		nodeNames = append(nodeNames, pod.Spec.NodeName)
	}
	return nodeNames, nil
}

func (v *PvcValidator) getResizeStorageWithOpsRequest(opsRequest *kbv1alpha1.OpsRequest) (int64, error) {
	opsVC := opsRequest.Spec.VolumeExpansionList[0]
	opsVCVolumeClaimTemplates := opsVC.VolumeClaimTemplates[0]
	cluster := &kbv1alpha1.Cluster{}
	if err := v.Client.Get(context.Background(), client.ObjectKey{
		Namespace: opsRequest.Namespace,
		Name:      opsRequest.Spec.ClusterRef,
	}, cluster); err != nil {
		return 0, fmt.Errorf("failed to get cluster: %w", err)
	}

	for _, cp := range cluster.Spec.ComponentSpecs {
		if cp.Name != opsVC.ComponentName {
			continue
		}
		for _, vc := range cp.VolumeClaimTemplates {
			if vc.Name != opsVCVolumeClaimTemplates.Name {
				continue
			}
			return opsVCVolumeClaimTemplates.Storage.Value() - vc.Spec.Resources.Requests.Storage().Value(), nil
		}
	}
	return 0, fmt.Errorf("not found volume claim template: %s", opsRequest.Spec.VolumeExpansionList[0].VolumeClaimTemplates[0].Name)
}
