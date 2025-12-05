package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/tools/clientcmd"
    
    // Import your local packages
    "intelligent-cluster-optimizer/pkg/metrics"
    "intelligent-cluster-optimizer/pkg/storage"
)

func main() {
    // 1. Setup Kubeconfig
	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatal(err)
	}

    // 2. Initialize Components
	collector, err := metrics.NewCollector(config)
	if err != nil {
		log.Fatal(err)
	}
    
    store := storage.NewStorage()

    // 3. Start the Loop (Ticker)
    ticker := time.NewTicker(10 * time.Second) // Poll every 10 seconds
    defer ticker.Stop()

    fmt.Println("Starting Metric Collector... (Press Ctrl+C to stop)")

    for range ticker.C {
        // A. Fetch
        data, err := collector.GetPodMetrics("workload-test") // Target your test namespace
        if err != nil {
            log.Printf("Error fetching metrics: %v", err)
            continue
        }

        // B. Store & Print
        fmt.Printf("[%s] Collecting metrics for %d pods...\n", time.Now().Format("15:04:05"), len(data))
        for _, pod := range data {
            store.Add(pod)
            fmt.Printf("   -> Pod: %s | CPU: %dm | Mem: %dMi\n", pod.PodName, pod.CPUMillis, pod.MemoryMB)
        }
    }
}