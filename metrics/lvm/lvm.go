package main

import (
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"github.com/labring/sealos/pkg/lvm"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"k8s.io/apimachinery/pkg/api/resource"
)

var (
	lvmVgsTotalCapacity = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "lvm_vgs_total_capacity",
		Help: "Total capacity of all volume groups in bytes",
	})

	lvmVgsTotalFree = prometheus.NewGauge(prometheus.GaugeOpts{
		Name: "lvm_vgs_total_free",
		Help: "Total free space of all volume groups in bytes",
	})
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
		UpdateInterval: 30,
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
	lvmVgsTotalCapacity.Set(float64(vgAmountTotal.Value()))
	lvmVgsTotalFree.Set(float64(vgFreeTotal.Value()))
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

func main() {
	http.Handle("/metrics", promhttp.Handler())

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

	log.Fatal(http.ListenAndServe(":9100", nil))
}
