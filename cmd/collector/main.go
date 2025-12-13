package main

import (
	"fmt"
	"log"
	"os"
	"path/filepath"
	"time"

	"k8s.io/client-go/tools/clientcmd"

	// Import local packages
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

	// 2. Read Configuration from Environment Variables (12-Factor App)
	namespace := os.Getenv("TARGET_NAMESPACE")
	if namespace == "" {
		namespace = "default" // Default namespace
	}

	pollIntervalStr := os.Getenv("POLL_INTERVAL")
	if pollIntervalStr == "" {
		pollIntervalStr = "30s" // Default poll interval
	}

	pollInterval, err := time.ParseDuration(pollIntervalStr)
	if err != nil {
		log.Fatalf("Invalid POLL_INTERVAL format '%s': %v. Use format like '30s', '1m', '2m30s'", pollIntervalStr, err)
	}

	fmt.Printf("Configuration:\n")
	fmt.Printf("  - Target Namespace: %s\n", namespace)
	fmt.Printf("  - Poll Interval: %s\n", pollInterval)
	fmt.Println()

	// 3. Initialize Components
	collector, err := metrics.NewCollector(config)
	if err != nil {
		log.Fatal(err)
	}

	// initialize storage
	store := storage.NewStorage()
	dataFile := "metrics_data.json"

	// load existing data on startup
	if err := store.LoadFromFile(dataFile); err != nil {
		log.Printf("Warning: Could not load old data: %v", err)
	} else {
		fmt.Println("Loaded historical data from disk.")
	}

	// save data automatically when the program exits
	defer func() {
		fmt.Println("Saving data to disk...")
		if err := store.SaveToFile(dataFile); err != nil {
			log.Printf("Error: Saving data: %v", err)
		} else {
			fmt.Println("Data saved successfully.")
		}
	}()

	go store.StartGarbageCollector(1*time.Hour, 24*time.Hour)

	// 4. Start the Loop (Ticker)
	ticker := time.NewTicker(pollInterval)
	defer ticker.Stop()

	fmt.Println("Starting Metric Collector... (Press Ctrl+C to stop)")

	for range ticker.C {
		// A. Fetch
		data, err := collector.GetPodMetrics(namespace)
		if err != nil {
			log.Printf("Error fetching metrics: %v", err)
			continue
		}

		// B. Store & Print
		fmt.Printf("[%s] Collecting metrics for %d pods...\n", time.Now().Format("15:04:05"), len(data))
		for _, pod := range data {
			// store the whole pod structure (includes containers)
			store.Add(pod)
			fmt.Printf("Pod: %s\n", pod.PodName)

			// loop through containers, where data is placed
			for _, container := range pod.Containers {
				fmt.Printf("   └─ %s\n", container.ContainerName)

				// print usage-request-limit 
				fmt.Printf("      Usage:   CPU: %4dm | Mem: %4dMi\n", container.UsageCPU, container.UsageMemory)
				fmt.Printf("      Request: CPU: %4dm | Mem: %4dMi\n", container.RequestCPU, container.RequestMemory)
				fmt.Printf("      Limit:   CPU: %4dm | Mem: %4dMi\n", container.LimitCPU, container.LimitMemory)
			}
		}
		fmt.Println("---------------------------------------------------")
	}
}