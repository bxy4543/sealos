package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"os"
	"time"

	launchpadServer "github.com/labring/sealos/service/launchpad/server"
	"github.com/labring/sealos/service/pkg/metrics"
	"github.com/labring/sealos/service/pkg/utils"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

type RestartableServer struct {
	configFile string
}

func (rs *RestartableServer) Serve(c *launchpadServer.Config) {
	vs, err := launchpadServer.NewVMServer(c)
	if err != nil {
		log.Fatalf("Failed to create auth server: %v", err)
	}
	mux := http.NewServeMux()
	mux.Handle("/", vs)
	mux.Handle("/metrics", promhttp.Handler())
	hs := &http.Server{
		Addr:    c.Server.ListenAddress,
		Handler: mux,
	}
	listener, err := net.Listen("tcp", c.Server.ListenAddress)
	if err != nil {
		log.Fatalf("Error creating listener: %v", err)
	}
	log.Printf("Serving on %s\n", c.Server.ListenAddress)

	if err := hs.Serve(listener); err != nil {
		log.Fatalf("Error starting server: %v", err)
	}
}

func main() {
	log.SetOutput(os.Stdout) // 将日志输出定向到标准输出（stdout）
	log.SetFlags(log.LstdFlags | log.Lshortfile)
	flag.Parse()
	configFile := flag.Arg(0)
	if configFile == "" {
		log.Fatal("Config file not specified")
	}
	config, err := launchpadServer.InitConfig(configFile)
	if err != nil {
		log.Fatalf("Error initializing config: %v", err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if config.VMLogsURL != "" {
		go startScrapeIngressCountTimer(ctx, config)
	}
	rs := RestartableServer{configFile: configFile}
	rs.Serve(config)
}

func startScrapeIngressCountTimer(ctx context.Context, config *launchpadServer.Config) {
	ticker := time.NewTicker(1 * time.Minute)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			log.Print("Starting scrape ingress count timer")
			if err := scrapeIngressCount(config); err != nil {
				log.Printf("Error scraping ingress count: %v", err)
			}
		case <-ctx.Done():
			log.Println("Scrape timer stopped")
			return
		}
	}
}

func scrapeIngressCount(config *launchpadServer.Config) error {
	metrics.IngressCounterClear()
	client := utils.NewClient(config.VMLogsURL, config.VMLogsUser, config.VMLogsPwd)
	response, err := client.QueryLogs(`namespace: "higress-system" app: "higress-gateway" | unpack_json | authority:(!="-" AND !="") route_name:(!="-" AND !="") | stats by(authority,route_name) count() as count | sort by(count) desc`, 1000000, time.Now().Add(-time.Minute), time.Now())
	if err != nil {
		return fmt.Errorf("failed to query logs: %w", err)
	}
	if err := metrics.UpdateMetricsWithVlogsData(response); err != nil {
		return fmt.Errorf("error updating metrics with vlogs data: %w", err)
	}
	return nil
}
