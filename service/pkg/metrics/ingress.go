package metrics

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strconv"

	"github.com/prometheus/client_golang/prometheus"
)

var (
	ingressCounter = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name: "ingress_count_total",
			Help: "Total count of logs by authority and route_name",
		},
		[]string{"authority", "route_name"},
	)
)

type LogEntry struct {
	Count     string `json:"count"`
	Authority string `json:"authority"`
	RouteName string `json:"route_name"`
}

func UpdateMetricsWithVlogsData(data []byte) error {
	logEntries, err := parseHigressLogEntryWithVlogsData(data)
	if err != nil {
		return fmt.Errorf("could not parse log entries: %s", err)
	}
	for _, entry := range logEntries {
		count, err := strconv.ParseFloat(entry.Count, 64)
		if err != nil {
			fmt.Println("Error converting count:", err)
			continue
		}
		ingressCounter.WithLabelValues(entry.Authority, entry.RouteName).Set(count)
	}
	return nil
}

func IngressCounterClear() {
	ingressCounter.Reset()
}

func parseHigressLogEntryWithVlogsData(data []byte) ([]LogEntry, error) {
	logEntries := []LogEntry{}
	lines := bytes.Split(data, []byte{'\n'})
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		var entry LogEntry
		if err := json.Unmarshal(line, &entry); err != nil {
			return nil, fmt.Errorf("could not parse log entry: %s", err)
		}
		logEntries = append(logEntries, entry)
	}
	return logEntries, nil
}

func init() {
	prometheus.MustRegister(ingressCounter)
}
