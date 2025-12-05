package metrics

import (
	"context"
	"intelligent-cluster-optimizer/pkg/models" // Import your local package
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/rest"
	metricsv "k8s.io/metrics/pkg/client/clientset/versioned"
)

type MetricsCollector struct {
	MetricsClient *metricsv.Clientset
}

func NewCollector(config *rest.Config) (*MetricsCollector, error) {
	metricsClient, err := metricsv.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return &MetricsCollector{MetricsClient: metricsClient}, nil
}

func (c *MetricsCollector) GetPodMetrics(namespace string) ([]models.PodMetric, error) {
	var results []models.PodMetric

	// Fetch from K8s
	metrics, err := c.MetricsClient.MetricsV1beta1().PodMetricses(namespace).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	// Transform K8s format to Our format
	for _, m := range metrics.Items {
		var totalCPU int64
		var totalMem int64

		var containers []models.ContainerMetric
		for _, container := range m.Containers {
			cpuMillis := container.Usage.Cpu().MilliValue()
			memoryMB := container.Usage.Memory().Value() / (1024 * 1024)

			containers = append(containers, models.ContainerMetric{
				ContainerName: container.Name,
				CPUMillis:     cpuMillis,
				MemoryMB:      memoryMB,
			})

			totalCPU += cpuMillis
			totalMem += memoryMB
		}

		results = append(results, models.PodMetric{
			PodName:    m.Name,
			Namespace:  m.Namespace,
			Timestamp:  time.Now(), // Or use m.Timestamp.Time
			Containers: containers,
			CPUMillis:  totalCPU,
			MemoryMB:   totalMem,
		})
	}
	return results, nil
}
