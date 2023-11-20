/*
Copyright 2023.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package v1

import (
	"bufio"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"reflect"
	"strings"

	"github.com/labring/sealos/controllers/pkg/utils"

	"k8s.io/apimachinery/pkg/api/resource"

	utilyaml "k8s.io/apimachinery/pkg/util/yaml"
	yaml2 "sigs.k8s.io/yaml"

	admissionv1 "k8s.io/api/admission/v1"

	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	"k8s.io/api/apps/v1beta2"
	"sigs.k8s.io/controller-runtime/pkg/client"
	logf "sigs.k8s.io/controller-runtime/pkg/log"

	kbv1alpha1 "github.com/apecloud/kubeblocks/apis/apps/v1alpha1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
)

var pvcLog = logf.Log.WithName("pvc-lvm-validating-webhook")

const ResizeAnnotation = "deploy.cloud.sealos.io/resize"

// +kubebuilder:rbac:groups=core,resources=pods,verbs=get;list
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=opsrequests;clusters,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps.kubeblocks.io,resources=opsrequests/status;clusters/status,verbs=get
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims,verbs=get;list;watch
// +kubebuilder:rbac:groups="",resources=persistentvolumeclaims/status,verbs=get
// +kubebuilder:rbac:groups=apps,resources=statefulsets,verbs=get;list;watch
// +kubebuilder:rbac:groups=apps,resources=statefulsets/status,verbs=get

// +kubebuilder:webhook:path=/validate-v1-sealos-pvc-check,mutating=false,failurePolicy=ignore,groups=apps.kubeblocks.io,resources=opsrequests;clusters,verbs=create;update,versions=v1,name=vpvccheck.kb.io,admissionReviewVersions=v1,sideEffects=None
// +kubebuilder:webhook:path=/validate-v1-sealos-pvc-check,mutating=false,failurePolicy=ignore,groups=apps,resources=statefulsets,verbs=create;update,versions=v1,name=vstatefulset.kb.io,admissionReviewVersions=v1,sideEffects=None
// +kubebuilder:webhook:path=/validate-v1-sealos-pvc-check,mutating=false,failurePolicy=ignore,groups="",resources=persistentvolumeclaims,verbs=create;update,versions=v1,name=vpvc.kb.io,admissionReviewVersions=v1,sideEffects=None
// +kubebuilder:object:generate=false

type PvcValidator struct {
	Client   client.Client
	PromoURL string
}

func (v *PvcValidator) Handle(ctx context.Context, req admission.Request) admission.Response {
	if req.Object.Object != nil {
		pvcLog.Error(errors.New("request object is not nil"), "", "req.Namespace", req.Namespace, "req.Name", req.Name, "req.gvrk", getGVRK(req), "req.Operation", req.Operation)
		return admission.Allowed("")
	}
	var err error
	switch req.Operation {
	case admissionv1.Delete:
		return admission.Allowed("")
	case admissionv1.Create:
		err = v.ValidateCreate(ctx, req.Object.Object)
	case admissionv1.Update:
		err = v.ValidateUpdate(req.Kind.Kind, req.OldObject, req.Object)
	}
	if err != nil {
		return admission.Denied(err.Error())
	}
	pvcLog.Info("pvc Handle", "req.Namespace", req.Namespace, "req.Name", req.Name, "req.gvrk", getGVRK(req), "req.Operation", req.Operation)
	return admission.Allowed("")
}

func (v *PvcValidator) ValidateCreate(_ context.Context, obj runtime.Object) error {
	ops, isKBOps := obj.(*kbv1alpha1.OpsRequest)
	if isKBOps && ops.Spec.Type == kbv1alpha1.VolumeExpansionType {
		return v.validateKBOpsRequest(ops)
	}
	pvcLog.Info("pvc ValidateCreate skip")
	return nil
}

func DecodeCluster(obj runtime.RawExtension) (*kbv1alpha1.Cluster, error) {
	reader := bytes.NewReader(obj.Raw)
	cluster := &kbv1alpha1.Cluster{}
	return cluster, unmarshalStrict(reader, cluster)
}

func DecodeOpsRequest(obj runtime.RawExtension) (*kbv1alpha1.OpsRequest, error) {
	reader := bytes.NewReader(obj.Raw)
	opsRequest := &kbv1alpha1.OpsRequest{}
	return opsRequest, unmarshalStrict(reader, opsRequest)
}

func DecodeStatefulSet(obj runtime.RawExtension) (*v1beta2.StatefulSet, error) {
	reader := bytes.NewReader(obj.Raw)
	statefulSet := &v1beta2.StatefulSet{}
	return statefulSet, unmarshalStrict(reader, statefulSet)
}

const nonStructPointerErrorFmt = "must be a struct pointer, got %T"

func unmarshalStrict(r io.Reader, obj interface{}) (err error) {
	if obj != nil && reflect.ValueOf(obj).Kind() != reflect.Pointer {
		return fmt.Errorf(nonStructPointerErrorFmt, obj)
	}
	if v := reflect.ValueOf(obj).Elem(); v.Kind() != reflect.Struct {
		return fmt.Errorf(nonStructPointerErrorFmt, obj)
	}

	rd := utilyaml.NewYAMLReader(bufio.NewReader(r))
	for {
		buf, rerr := rd.Read()
		if rerr == io.EOF {
			break
		}
		if rerr != nil {
			return rerr
		}
		if len(bytes.TrimSpace(buf)) == 0 {
			continue
		}
		if err = yaml2.UnmarshalStrict(buf, obj); err == nil {
			return nil
		}
	}
	if err != nil {
		if strings.Contains(err.Error(), "json: unknown field") {
			err = fmt.Errorf("document do not have corresponding struct %T", obj)
		}
	}
	return
}

func (v *PvcValidator) ValidateUpdate(kind string, oldObj, newObj runtime.RawExtension) error {
	switch kind {
	case "Cluster":
		oldCluster, err := DecodeCluster(oldObj)
		if err != nil {
			return fmt.Errorf("failed to decode old cluster: %w", err)
		}
		newCluster, err := DecodeCluster(newObj)
		if err != nil {
			return fmt.Errorf("failed to decode new cluster: %w", err)
		}
		return v.validateKBCluster(oldCluster, newCluster)
	case "OpsRequest":
		newOps, err := DecodeOpsRequest(newObj)
		if err != nil {
			return fmt.Errorf("failed to decode new ops request: %w", err)
		}
		return v.validateKBOpsRequest(newOps)
	case "StatefulSet":
		newSts, err := DecodeStatefulSet(newObj)
		if err != nil {
			return fmt.Errorf("failed to decode new stateful set: %w", err)
		}
		return v.validateStatefulSet(newSts)
	default:
		return nil
	}
}

func (v *PvcValidator) validateKBCluster(oldCluster, newCluster *kbv1alpha1.Cluster) error {
	expansionSize := newCluster.Spec.ComponentSpecs[0].VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Value() - oldCluster.Spec.ComponentSpecs[0].VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Value()
	if expansionSize < 0 {
		return fmt.Errorf("cluster can not be scaled down")
	}
	if expansionSize == 0 {
		return nil
	}
	nodeNames, err := v.getPodNodeName(newCluster.Namespace, client.MatchingLabels{"app.kubernetes.io/instance": newCluster.Name, "app.kubernetes.io/managed-by": "kubeblocks"})
	if err != nil {
		return fmt.Errorf("failed to get sts pod node name: %w", err)
	}
	pvcLog.Info("pvc validateKBCluster", "namespace", newCluster.Namespace, "pvc name", newCluster.Name, "expansionSize", expansionSize)
	return utils.CheckResourceShortageError(v.checkStorageCapacity(nodeNames, expansionSize, newCluster.Namespace, newCluster.Name))
}

func (v *PvcValidator) validateKBOpsRequest(opsRequest *kbv1alpha1.OpsRequest) error {
	if opsRequest.Spec.VolumeExpansionList == nil {
		return nil
	}

	//app.kubernetes.io/instance=test-name,app.kubernetes.io/managed-by=kubeblocks
	nodeNames, err := v.getPodNodeName(opsRequest.Namespace, client.MatchingLabels{"app.kubernetes.io/instance": opsRequest.Spec.ClusterRef, "app.kubernetes.io/managed-by": "kubeblocks"})
	if err != nil {
		return fmt.Errorf("failed to get sts pod node name: %w", err)
	}
	expansionSize, err := v.getResizeStorageWithOpsRequest(opsRequest)
	if err != nil {
		return fmt.Errorf("failed to get db storage with ops request: %w", err)
	}

	pvcLog.Info("pvc validateKBOpsRequest", "namespace", opsRequest.Namespace, "pvc name", opsRequest.Name, "expansionSize", expansionSize)
	return utils.CheckResourceShortageError(v.checkStorageCapacity(nodeNames, expansionSize, opsRequest.Namespace, opsRequest.Name))
}

func (v *PvcValidator) validateStatefulSet(newSts *v1beta2.StatefulSet) error {
	if len(newSts.Spec.VolumeClaimTemplates) == 0 || newSts.GetLabels() == nil {
		return nil
	}
	resizeStr := newSts.GetAnnotations()[ResizeAnnotation]
	if resizeStr == "" {
		pvcLog.Info("pvc resize annotation is empty", "namespace", newSts.Namespace, "pvc name", newSts.Name)
		return nil
	}
	resize := resource.MustParse(resizeStr)
	expansionSize := resize.Value() - newSts.Spec.VolumeClaimTemplates[0].Spec.Resources.Requests.Storage().Value()

	podList, err := v.getPodNodeName(newSts.Namespace, newSts.Spec.Selector.MatchLabels)
	if err != nil {
		return fmt.Errorf("failed to get sts pod node name: %w", err)
	}
	return utils.CheckResourceShortageError(v.checkStorageCapacity(podList,
		expansionSize,
		newSts.Namespace, newSts.Name))
}

func (v *PvcValidator) checkStorageCapacity(nodeNames []string, requestedStorage int64, namespace, name string) error {
	pvcLog.Info("check storage capacity", "namespace", namespace, "pvc name", name, "nodeNames", nodeNames, "requestedStorage", requestedStorage)
	for _, nodeName := range nodeNames {
		if nodeName == "" {
			continue
		}
		residualStorage, err := v.newLVMVgTotalFreeQuery(nodeName)
		if err != nil {
			return fmt.Errorf("failed to get lvm vgs total free: %w", err)
		}

		pvcLog.Info("check storage capacity", "namespace", namespace, "pvc name", name, "nodeName", nodeName, "residualStorage", residualStorage, "requestedStorage", requestedStorage)
		if residualStorage < requestedStorage {
			pvcLog.Error(fmt.Errorf("pvc can not be scaled up"), "", "namespace", namespace, "pvc name", name, "nodeName", nodeName, "residualStorage", residualStorage, "requestedStorage", requestedStorage)
			return utils.NewResourceShortageError(fmt.Errorf("pvc %s/%s can not be scaled down", namespace, name))
		}
	}
	return nil
}

func (v *PvcValidator) newLVMVgTotalFreeQuery(_ string) (int64, error) {
	// hack 999G数据
	return 1 * 1024 * 1024 * 1024, nil
	//prom, err := prometheus.NewPrometheus(v.PromoURL)
	//if err != nil {
	//	return 0, err
	//}
	//residualSize, err := prom.QueryLvmVgsTotalFree(prometheus.QueryParams{
	//	Node: node,
	//})
	//if err != nil {
	//	return 0, fmt.Errorf("failed to query lvm vgs total free: %w", err)
	//}
	//return int64(residualSize), nil
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

func getGVRK(req admission.Request) string {
	if req.Kind.Group == "" {
		return fmt.Sprintf("%s.%s/%s", req.Resource.Resource, req.Kind.Kind, req.Kind.Version)
	}
	return fmt.Sprintf("%s.%s.%s/%s", req.Resource.Resource, req.Kind.Kind, req.Kind.Group, req.Kind.Version)
}
