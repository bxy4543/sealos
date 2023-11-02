package prometheus

import (
	"fmt"
	"testing"
)

func Test_prometheus_QueryLvmVgsTotalFree(t *testing.T) {
	prom, err := NewPrometheus("http://localhost:9092")
	if err != nil {
		t.Fatal(err)
	}
	value, err := prom.QueryLvmVgsTotalFree(QueryParams{
		Tmpl:  VgsTotalFree,
		Range: nil,
		Node:  "node-1",
	})
	if err != nil {
		t.Fatal(fmt.Errorf("failed to query lvm vgs total free: %w", err))
	}
	fmt.Println("value: ", value)

	fmt.Println(formatBytes(value))

}

func formatBytes(bytes float64) string {
	const (
		_          = iota
		KB float64 = 1 << (10 * iota)
		MB
		GB
		TB
	)

	switch {
	case bytes < KB:
		return fmt.Sprintf("%.2f B", bytes)
	case bytes < MB:
		return fmt.Sprintf("%.2f KB", bytes/KB)
	case bytes < GB:
		return fmt.Sprintf("%.2f MB", bytes/MB)
	case bytes < TB:
		return fmt.Sprintf("%.2f GB", bytes/GB)
	default:
		return fmt.Sprintf("%.2f TB", bytes/TB)
	}
}
