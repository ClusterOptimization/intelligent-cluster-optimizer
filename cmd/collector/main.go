package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"path/filepath"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

func main() {
	kubeconfig := filepath.Join(os.Getenv("HOME"), ".kube", "config")
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		log.Fatal(err)
	}

	// regular client for pod info
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		log.Fatal(err)
	}

	// metrics client for cpu/memory
	metricsClient, err := metricsv.NewForConfig(config)
    if err != nil {
        log.Fatal(err)
    }

	// get pods
	pods, err := clientset.CoreV1().Pods("workload-test").List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		log.Fatal(err)
	}

	fmt.Printf("Found %d pods:\n", len(pods.Items))

	// get metrics for each pod
    for _, pod := range pods.Items {
        metrics, err := metricsClient.MetricsV1beta1().PodMetricses("workload-test").Get(context.TODO(), pod.Name, metav1.GetOptions{})
        if err != nil {
            fmt.Printf("- %s: (metrics not available)\n", pod.Name)
            continue
        }

        fmt.Printf("Pod: %s\n", pod.Name)
        for _, container := range metrics.Containers {
            cpu := container.Usage.Cpu().MilliValue()
            memory := container.Usage.Memory().Value() / (1024 * 1024) // Convert to MB
            fmt.Printf("  CPU: %dm, Memory: %dMi\n", cpu, memory)
        }
        fmt.Println()
    }
}
