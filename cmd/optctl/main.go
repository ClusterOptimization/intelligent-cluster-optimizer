package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"text/tabwriter"
	"time"

	"intelligent-cluster-optimizer/pkg/rollback"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	kubeconfig  string
	container   string
	historyFile string
	outputJSON  bool
)

const defaultHistoryFile = "/var/lib/optimizer/rollback-history.json"

func main() {
	klog.InitFlags(nil)
	flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "Path to kubeconfig")
	flag.StringVar(&container, "container", "", "Container name (default: first container)")
	flag.StringVar(&historyFile, "history-file", defaultHistoryFile, "Path to rollback history file")
	flag.BoolVar(&outputJSON, "json", false, "Output in JSON format")
	flag.Parse()

	if len(flag.Args()) < 1 {
		printUsage()
		os.Exit(1)
	}

	command := flag.Args()[0]

	// History command doesn't need kubernetes client
	if command == "history" {
		if err := handleHistory(); err != nil {
			klog.Fatalf("History command failed: %v", err)
		}
		return
	}

	// Other commands need resource argument
	if len(flag.Args()) < 2 {
		printUsage()
		os.Exit(1)
	}

	resource := flag.Args()[1]

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to build config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create kubernetes client: %v", err)
	}

	switch command {
	case "rollback":
		if err := handleRollback(kubeClient, resource); err != nil {
			klog.Fatalf("Rollback failed: %v", err)
		}
	default:
		klog.Fatalf("Unknown command: %s", command)
	}
}

func printUsage() {
	fmt.Fprintf(os.Stderr, "Usage: optctl <command> [options]\n")
	fmt.Fprintf(os.Stderr, "\nCommands:\n")
	fmt.Fprintf(os.Stderr, "  history [resource]                    Show optimization history\n")
	fmt.Fprintf(os.Stderr, "  rollback <namespace/kind/name>        Rollback workload to previous config\n")
	fmt.Fprintf(os.Stderr, "\nOptions:\n")
	fmt.Fprintf(os.Stderr, "  --kubeconfig      Path to kubeconfig (default: ~/.kube/config)\n")
	fmt.Fprintf(os.Stderr, "  --container       Container name (default: first container)\n")
	fmt.Fprintf(os.Stderr, "  --history-file    Path to history file (default: %s)\n", defaultHistoryFile)
	fmt.Fprintf(os.Stderr, "  --json            Output in JSON format\n")
	fmt.Fprintf(os.Stderr, "\nExamples:\n")
	fmt.Fprintf(os.Stderr, "  optctl history                                      # Show all history\n")
	fmt.Fprintf(os.Stderr, "  optctl history default/Deployment/nginx             # Show history for workload\n")
	fmt.Fprintf(os.Stderr, "  optctl rollback default/Deployment/nginx            # Rollback workload\n")
	fmt.Fprintf(os.Stderr, "  optctl rollback prod/StatefulSet/redis --container=redis\n")
}

func handleHistory() error {
	manager := rollback.NewRollbackManager(nil)

	// Load history from file
	if err := manager.LoadFromFile(historyFile); err != nil {
		return fmt.Errorf("failed to load history: %v", err)
	}

	history := manager.GetAllHistory()

	if len(history) == 0 {
		fmt.Println("No optimization history found.")
		fmt.Printf("History file: %s\n", historyFile)
		return nil
	}

	// Check if specific workload requested
	if len(flag.Args()) > 1 {
		return showWorkloadHistory(manager, flag.Args()[1])
	}

	// Show all history
	return showAllHistory(history)
}

func showAllHistory(history map[string][]rollback.WorkloadConfig) error {
	// Collect all entries for sorting
	type historyEntry struct {
		key       string
		config    rollback.WorkloadConfig
		entryNum  int
		totalHist int
	}

	var entries []historyEntry
	for key, configs := range history {
		for i, config := range configs {
			entries = append(entries, historyEntry{
				key:       key,
				config:    config,
				entryNum:  i + 1,
				totalHist: len(configs),
			})
		}
	}

	// Sort by timestamp (newest first)
	sort.Slice(entries, func(i, j int) bool {
		return entries[i].config.Timestamp.After(entries[j].config.Timestamp)
	})

	// Print summary
	fmt.Printf("Optimization History (%d entries across %d workloads)\n", len(entries), len(history))
	fmt.Println(strings.Repeat("-", 80))

	// Create tabwriter for aligned output
	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "WORKLOAD\tCONTAINER\tCPU\tMEMORY\tTIMESTAMP\tAGE")

	for _, entry := range entries {
		age := formatAge(time.Since(entry.config.Timestamp))
		workload := fmt.Sprintf("%s/%s/%s",
			entry.config.Namespace,
			entry.config.Kind,
			entry.config.Name)

		fmt.Fprintf(w, "%s\t%s\t%s\t%s\t%s\t%s\n",
			workload,
			entry.config.ContainerName,
			entry.config.CPU,
			entry.config.Memory,
			entry.config.Timestamp.Format("2006-01-02 15:04"),
			age)
	}

	return w.Flush()
}

func showWorkloadHistory(manager *rollback.RollbackManager, resource string) error {
	parts := strings.Split(resource, "/")
	if len(parts) < 3 {
		return fmt.Errorf("invalid resource format, expected: namespace/kind/name[/container]")
	}

	namespace := parts[0]
	kind := parts[1]
	name := parts[2]

	containerName := container
	if len(parts) > 3 {
		containerName = parts[3]
	}
	if containerName == "" {
		containerName = "app" // default
	}

	configs := manager.GetWorkloadHistory(namespace, kind, name, containerName)

	if len(configs) == 0 {
		fmt.Printf("No history found for %s/%s/%s (container: %s)\n", namespace, kind, name, containerName)
		return nil
	}

	fmt.Printf("History for %s/%s/%s (container: %s)\n", namespace, kind, name, containerName)
	fmt.Println(strings.Repeat("-", 60))

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "#\tCPU\tMEMORY\tTIMESTAMP\tAGE")

	for i, config := range configs {
		age := formatAge(time.Since(config.Timestamp))
		fmt.Fprintf(w, "%d\t%s\t%s\t%s\t%s\n",
			i+1,
			config.CPU,
			config.Memory,
			config.Timestamp.Format("2006-01-02 15:04:05"),
			age)
	}

	if err := w.Flush(); err != nil {
		return err
	}

	fmt.Println()
	fmt.Printf("Total entries: %d (max: %d per workload)\n", len(configs), rollback.MaxHistoryPerWorkload)

	if len(configs) >= 2 {
		fmt.Println("\nTo rollback to previous configuration:")
		fmt.Printf("  optctl rollback %s/%s/%s --container=%s\n", namespace, kind, name, containerName)
	}

	return nil
}

func handleRollback(kubeClient kubernetes.Interface, resource string) error {
	parts := strings.Split(resource, "/")
	if len(parts) != 3 {
		return fmt.Errorf("invalid resource format, expected: namespace/kind/name")
	}

	namespace := parts[0]
	kind := parts[1]
	name := parts[2]

	if container == "" {
		container = "app"
		klog.Infof("No container specified, using default: %s", container)
	}

	manager := rollback.NewRollbackManager(kubeClient)

	// Load existing history
	if err := manager.LoadFromFile(historyFile); err != nil {
		klog.Warningf("Could not load history file: %v", err)
	}

	klog.Infof("Rolling back %s/%s/%s container=%s", namespace, kind, name, container)

	ctx := context.Background()
	if err := manager.RollbackWorkload(ctx, namespace, kind, name, container); err != nil {
		return err
	}

	// Save updated history
	if err := manager.SaveToFile(historyFile); err != nil {
		klog.Warningf("Could not save history file: %v", err)
	}

	fmt.Printf("Successfully rolled back %s/%s/%s\n", namespace, kind, name)
	return nil
}

func formatAge(d time.Duration) string {
	if d < time.Minute {
		return fmt.Sprintf("%ds", int(d.Seconds()))
	}
	if d < time.Hour {
		return fmt.Sprintf("%dm", int(d.Minutes()))
	}
	if d < 24*time.Hour {
		return fmt.Sprintf("%dh", int(d.Hours()))
	}
	days := int(d.Hours() / 24)
	return fmt.Sprintf("%dd", days)
}
