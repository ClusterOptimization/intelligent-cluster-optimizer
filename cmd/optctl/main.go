package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"intelligent-cluster-optimizer/pkg/rollback"

	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
)

var (
	kubeconfig string
	container  string
)

func main() {
	klog.InitFlags(nil)
	flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "Path to kubeconfig")
	flag.StringVar(&container, "container", "", "Container name (default: first container)")
	flag.Parse()

	if len(flag.Args()) < 2 {
		fmt.Fprintf(os.Stderr, "Usage: optctl <command> <resource>\n")
		fmt.Fprintf(os.Stderr, "\nCommands:\n")
		fmt.Fprintf(os.Stderr, "  rollback <namespace>/<kind>/<name>    Rollback workload to previous config\n")
		fmt.Fprintf(os.Stderr, "\nExamples:\n")
		fmt.Fprintf(os.Stderr, "  optctl rollback default/Deployment/nginx\n")
		fmt.Fprintf(os.Stderr, "  optctl rollback prod/StatefulSet/redis --container=redis\n")
		os.Exit(1)
	}

	command := flag.Args()[0]
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

func handleRollback(kubeClient kubernetes.Interface, resource string) error {
	parts := strings.Split(resource, "/")
	if len(parts) != 3 {
		return fmt.Errorf("invalid resource format, expected: namespace/kind/name")
	}

	namespace := parts[0]
	kind := parts[1]
	name := parts[2]

	ctx := context.Background()

	if container == "" {
		container = "app"
		klog.Infof("No container specified, using default: %s", container)
	}

	manager := rollback.NewRollbackManager(kubeClient)

	klog.Infof("Rolling back %s/%s/%s container=%s", namespace, kind, name, container)

	if err := manager.RollbackWorkload(ctx, namespace, kind, name, container); err != nil {
		return err
	}

	fmt.Printf("Successfully rolled back %s/%s/%s\n", namespace, kind, name)
	return nil
}
