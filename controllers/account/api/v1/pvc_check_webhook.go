package v1

import (
	"bufio"
	"bytes"
	"context"
	"fmt"
	"io"
	"reflect"
	"strconv"
	"strings"

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

type PvcValidator struct {
	client.Client
	PromoURL string
}

func (v *PvcValidator) Handle(ctx context.Context, req admission.Request) error {
	var err error
	logger.Info("pvc Handle", "req.Object", req.Object.Object.GetObjectKind().GroupVersionKind())
	switch req.Operation {
	case admissionv1.Create:
		err = v.ValidateCreate(ctx, req.Object.Object)
	case admissionv1.Update:
		logger.Info("pvc Handle Update", "req.OldObject", req.OldObject.Object.GetObjectKind().GroupVersionKind())
		err = v.ValidateUpdate(ctx, req.Object, req.OldObject)
	}

	if err != nil {
		return fmt.Errorf("failed to validate pvc: %w", err)
	}
	logger.Info("pvc Handle Success", "req.Namespace", req.Namespace, "req.Name", req.Name, "req.gvrk", getGVRK(req), "req.Operation", req.Operation)
	return nil
}

func (v *PvcValidator) ValidateCreate(_ context.Context, obj runtime.Object) error {
	ops, isKBOps := obj.(*kbv1alpha1.OpsRequest)
	if isKBOps && ops.Spec.Type == kbv1alpha1.VolumeExpansionType {
		return v.validateKBOpsRequest(ops)
	}
	logger.Info("pvc ValidateCreate skip")
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

func (v *PvcValidator) ValidateUpdate(_ context.Context, oldObj, newObj runtime.RawExtension) error {
	switch newObj.Object.GetObjectKind().GroupVersionKind().Kind {
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
		oldSts, err := DecodeStatefulSet(oldObj)
		if err != nil {
			return fmt.Errorf("failed to decode old stateful set: %w", err)
		}
		newSts, err := DecodeStatefulSet(newObj)
		if err != nil {
			return fmt.Errorf("failed to decode new stateful set: %w", err)
		}
		return v.validateStatefulSet(oldSts, newSts)
	default:
		return fmt.Errorf("not support kind: %s", newObj.Object.GetObjectKind().GroupVersionKind().Kind)
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
	err = v.checkStorageCapacity(nodeNames, expansionSize, newCluster.Namespace, newCluster.Name)
	if err != nil {
		return fmt.Errorf("failed to check db storage capacity: %w", err)
	}
	logger.Info("pvc validateKBCluster Success", "namespace", newCluster.Namespace, "pvc name", newCluster.Name, "expansionSize", expansionSize)
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
		return fmt.Errorf("failed to get db storage with ops request: %w", err)
	}

	err = v.checkStorageCapacity(nodeNames, expansionSize, opsRequest.Namespace, opsRequest.Name)
	if err != nil {
		return fmt.Errorf("failed to check db storage capacity: %w", err)
	}
	logger.Info("pvc validateKBOpsRequest Success", "namespace", opsRequest.Namespace, "pvc name", opsRequest.Name, "expansionSize", expansionSize)
	return nil
}

func (v *PvcValidator) validateStatefulSet(_, newSts *v1beta2.StatefulSet) error {
	resizeStr := newSts.GetLabels()["resize"]
	if resizeStr == "" {
		logger.Info("pvc resize label is empty", "namespace", newSts.Namespace, "pvc name", newSts.Name)
		return nil
	}
	resize, err := strconv.ParseInt(resizeStr, 10, 64)
	if err != nil {
		return fmt.Errorf("failed to convert resize label to int: %w", err)
	}

	podList, err := v.getPodNodeName(newSts.Namespace, newSts.Spec.Selector.MatchLabels)
	if err != nil {
		return fmt.Errorf("failed to get sts pod node name: %w", err)
	}
	err = v.checkStorageCapacity(podList,
		resize,
		newSts.Namespace, newSts.Name)
	if err != nil {
		return fmt.Errorf("failed to check storage capacity: %w", err)
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
		pvcLog.Info("check storage capacity", "namespace", namespace, "pvc name", name, "nodeName", nodeName, "residualStorage", residualStorage, "requestedStorage", requestedStorage)
		if residualStorage < requestedStorage {
			pvcLog.Error(fmt.Errorf("failed to scaled down pvc"), "pvc can not be scaled up", "namespace", namespace, "pvc name", name, "nodeName", nodeName, "residualStorage", residualStorage, "requestedStorage", requestedStorage)
			return fmt.Errorf("pvc %s/%s can not be scaled down", namespace, name)
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
