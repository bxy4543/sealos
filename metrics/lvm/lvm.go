package main

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"

	"github.com/labring/sealos/pkg/lvm"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	lvmVgsTotalCapacity = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lvm_vgs_total_capacity",
			Help: "Total capacity of all volume groups in bytes",
		},
		[]string{"node"},
	)

	lvmVgsTotalFree = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "lvm_vgs_total_free",
			Help: "Total free space of all volume groups in bytes",
		},
		[]string{"node"},
	)
)

func init() {
	prometheus.MustRegister(lvmVgsTotalCapacity)
	prometheus.MustRegister(lvmVgsTotalFree)
}

type MetricsCollector struct {
	UpdateInterval int
}

func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		UpdateInterval: 10,
	}
}

func (mc *MetricsCollector) updateMetrics() {
	vgs, err := lvm.ListLVMVolumeGroup(false)
	if err != nil {
		log.Printf("Failed to list LVM volume groups: %v", err)
		return
	}
	vgAmountTotal := resource.NewQuantity(0, resource.BinarySI)
	vgFreeTotal := resource.NewQuantity(0, resource.BinarySI)
	for _, vg := range vgs {
		vgAmountTotal.Add(vg.Size)
		vgFreeTotal.Add(vg.Free)
	}
	nodeName := os.Getenv("NODE_NAME")
	lvmVgsTotalCapacity.With(prometheus.Labels{"node": nodeName}).Set(float64(vgAmountTotal.Value()))
	lvmVgsTotalFree.With(prometheus.Labels{"node": nodeName}).Set(float64(vgFreeTotal.Value()))
	log.Printf("Updated LVM metrics: total capacity %v, total free %v\n", vgAmountTotal, vgFreeTotal)
}

func (mc *MetricsCollector) startMetricsCollection() {
	go func() {
		for {
			mc.updateMetrics()
			time.Sleep(time.Duration(mc.UpdateInterval) * time.Second)
		}
	}()
}

func startRestoreCurrentNodePvcTask(s string) {
	if s == "" {
		s = "1h"
	}
	fmt.Println("startRestoreCurrentNodePvcTask ", s)
	ts, err := time.ParseDuration(s)
	if err != nil {
		log.Fatalf("Failed to parse duration: %v", err)
		return
	}
	ticker := time.NewTicker(ts)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			lvs, err := RestoreCurrentNodePvc()
			if err != nil {
				log.Printf("Error restoring PVCs: %v", err)
			} else {
				log.Printf("Restored PVCs: %v", lvs)
			}
		}
	}
}

func main() {
	http.Handle("/metrics", promhttp.Handler())
	http.Handle("/restore-pvc-size", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		lvs, err := RestoreCurrentNodePvc()
		if err != nil {
			w.WriteHeader(500)
			w.Write([]byte(err.Error()))
			return
		}
		w.WriteHeader(200)
		w.Write([]byte(fmt.Sprint("Restored PVCs: ", lvs)))
	}))

	mc := NewMetricsCollector()
	updateIntervalStr := os.Getenv("UPDATE_INTERVAL")
	if updateIntervalStr != "" {
		updateInterval, err := strconv.Atoi(updateIntervalStr)
		if err != nil {
			log.Fatalf("UPDATE_INTERVAL must be a number, got %v", updateIntervalStr)
		}
		mc.UpdateInterval = updateInterval
	}
	mc.startMetricsCollection()

	go startRestoreCurrentNodePvcTask(os.Getenv("RESTORE_PVC_SIZE_INTERVAL"))

	log.Fatal(http.ListenAndServe(":9100", nil))
}

func RestoreCurrentNodePvc() ([]string, error) {
	nodeName := os.Getenv("NODE_NAME")
	// Get the PVCs on the node

	conf, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	clt, err := kubernetes.NewForConfig(conf)
	if err != nil {
		return nil, fmt.Errorf("failed to create k8s client: %v", err)
	}

	pvcs, err := clt.CoreV1().PersistentVolumeClaims("").List(context.TODO(), metav1.ListOptions{})

	if err != nil {
		return nil, fmt.Errorf("failed to list PVCs: %v", err)
	}

	var pvcNames = make(map[string]v1.PersistentVolumeClaim)
	for i, pvc := range pvcs.Items {
		if pvc.Status.Phase != "Bound" {
			continue
		}
		if pvc.Annotations == nil || pvc.Annotations["volume.kubernetes.io/selected-node"] != nodeName {
			continue
		}

		specSize, statusSize := pvc.Spec.Resources.Requests.Storage(), pvc.Status.Capacity.Storage()
		if specSize.Value() != statusSize.Value() {
			continue
		}
		pvcNames[pvc.Spec.VolumeName] = pvcs.Items[i]
	}

	//fmt.Printf("pvcNames %v\n", pvcNames)

	lvs, err := lvm.ListLVMLogicalVolume()
	if err != nil {
		return nil, fmt.Errorf("failed to list LVM logical volumes: %v", err)
	}
	resizeVolume := map[string]struct{}{}
	for i := range lvs {
		if lvs[i].Size == 0 {
			continue
		}
		if pvc, ok := pvcNames[lvs[i].Name]; ok {
			// pvc的byte size等于lvs size跳过
			if lvs[i].Size >= pvc.Status.Capacity.Storage().Value() {
				continue
			}
			if _, ok2 := resizeVolume[lvs[i].Name]; ok2 {
				continue
			}
			// resize pvc
			err = lvm.ResizeVolume(lvs[i], strconv.FormatInt(pvc.Status.Capacity.Storage().Value(), 10), true)
			if err != nil {
				return nil, fmt.Errorf("failed to resize PVC: %v", err)
			}
			fmt.Printf("resize pvc %s %s :  lvs size %s , pvc size %s \n", pvc.Namespace, lvs[i].Name, strconv.FormatInt(lvs[i].Size, 10), strconv.FormatInt(pvc.Status.Capacity.Storage().Value(), 10))
			resizeVolume[lvs[i].Name] = struct{}{}
		}
	}
	var resizeVolumeNames []string
	for k := range resizeVolume {
		resizeVolumeNames = append(resizeVolumeNames, k)
	}
	return resizeVolumeNames, nil
}
