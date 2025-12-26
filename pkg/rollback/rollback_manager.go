package rollback

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sync"
	"time"

	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

const MaxHistoryPerWorkload = 5

type RollbackManager struct {
	mu         sync.RWMutex
	kubeClient kubernetes.Interface
	history    map[string][]WorkloadConfig
}

func NewRollbackManager(kubeClient kubernetes.Interface) *RollbackManager {
	return &RollbackManager{
		kubeClient: kubeClient,
		history:    make(map[string][]WorkloadConfig),
	}
}

func (r *RollbackManager) SavePreviousConfig(ctx context.Context, namespace, kind, name, containerName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	config, err := r.fetchCurrentConfig(ctx, namespace, kind, name, containerName)
	if err != nil {
		return fmt.Errorf("failed to fetch current config: %v", err)
	}

	key := config.Key()
	configs := r.history[key]
	configs = append(configs, *config)

	if len(configs) > MaxHistoryPerWorkload {
		configs = configs[len(configs)-MaxHistoryPerWorkload:]
	}

	r.history[key] = configs
	klog.V(3).Infof("Saved config for %s (total history: %d)", key, len(configs))
	return nil
}

func (r *RollbackManager) RollbackWorkload(ctx context.Context, namespace, kind, name, containerName string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	key := namespace + "/" + kind + "/" + name + "/" + containerName
	configs := r.history[key]

	if len(configs) < 2 {
		return fmt.Errorf("no previous config available for %s", key)
	}

	previousConfig := configs[len(configs)-2]

	klog.Infof("Rolling back %s to CPU=%s Memory=%s", key, previousConfig.CPU, previousConfig.Memory)

	if err := r.applyConfig(ctx, &previousConfig); err != nil {
		return fmt.Errorf("failed to apply rollback: %v", err)
	}

	r.history[key] = configs[:len(configs)-1]
	klog.Infof("Successfully rolled back %s", key)
	return nil
}

func (r *RollbackManager) fetchCurrentConfig(ctx context.Context, namespace, kind, name, containerName string) (*WorkloadConfig, error) {
	config := &WorkloadConfig{
		Namespace:     namespace,
		Kind:          kind,
		Name:          name,
		ContainerName: containerName,
		Timestamp:     time.Now(),
	}

	var containers []corev1.Container

	switch kind {
	case "Deployment":
		deploy, err := r.kubeClient.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		containers = deploy.Spec.Template.Spec.Containers

	case "StatefulSet":
		sts, err := r.kubeClient.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		containers = sts.Spec.Template.Spec.Containers

	case "DaemonSet":
		ds, err := r.kubeClient.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, err
		}
		containers = ds.Spec.Template.Spec.Containers

	default:
		return nil, fmt.Errorf("unsupported kind: %s", kind)
	}

	for _, container := range containers {
		if container.Name == containerName {
			if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
				config.CPU = cpu.String()
			}
			if mem, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
				config.Memory = mem.String()
			}
			return config, nil
		}
	}

	return nil, fmt.Errorf("container %s not found", containerName)
}

func (r *RollbackManager) applyConfig(ctx context.Context, config *WorkloadConfig) error {
	switch config.Kind {
	case "Deployment":
		return r.rollbackDeployment(ctx, config)
	case "StatefulSet":
		return r.rollbackStatefulSet(ctx, config)
	case "DaemonSet":
		return r.rollbackDaemonSet(ctx, config)
	default:
		return fmt.Errorf("unsupported kind: %s", config.Kind)
	}
}

func (r *RollbackManager) rollbackDeployment(ctx context.Context, config *WorkloadConfig) error {
	deploy, err := r.kubeClient.AppsV1().Deployments(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for i := range deploy.Spec.Template.Spec.Containers {
		if deploy.Spec.Template.Spec.Containers[i].Name == config.ContainerName {
			if err := r.updateContainerResources(&deploy.Spec.Template.Spec.Containers[i], config); err != nil {
				return err
			}
			break
		}
	}

	_, err = r.kubeClient.AppsV1().Deployments(config.Namespace).Update(ctx, deploy, metav1.UpdateOptions{})
	return err
}

func (r *RollbackManager) rollbackStatefulSet(ctx context.Context, config *WorkloadConfig) error {
	sts, err := r.kubeClient.AppsV1().StatefulSets(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for i := range sts.Spec.Template.Spec.Containers {
		if sts.Spec.Template.Spec.Containers[i].Name == config.ContainerName {
			if err := r.updateContainerResources(&sts.Spec.Template.Spec.Containers[i], config); err != nil {
				return err
			}
			break
		}
	}

	_, err = r.kubeClient.AppsV1().StatefulSets(config.Namespace).Update(ctx, sts, metav1.UpdateOptions{})
	return err
}

func (r *RollbackManager) rollbackDaemonSet(ctx context.Context, config *WorkloadConfig) error {
	ds, err := r.kubeClient.AppsV1().DaemonSets(config.Namespace).Get(ctx, config.Name, metav1.GetOptions{})
	if err != nil {
		return err
	}

	for i := range ds.Spec.Template.Spec.Containers {
		if ds.Spec.Template.Spec.Containers[i].Name == config.ContainerName {
			if err := r.updateContainerResources(&ds.Spec.Template.Spec.Containers[i], config); err != nil {
				return err
			}
			break
		}
	}

	_, err = r.kubeClient.AppsV1().DaemonSets(config.Namespace).Update(ctx, ds, metav1.UpdateOptions{})
	return err
}

func (r *RollbackManager) updateContainerResources(container *corev1.Container, config *WorkloadConfig) error {
	if container.Resources.Requests == nil {
		container.Resources.Requests = corev1.ResourceList{}
	}

	if config.CPU != "" {
		cpuQuantity, err := resource.ParseQuantity(config.CPU)
		if err != nil {
			return fmt.Errorf("invalid CPU quantity: %v", err)
		}
		container.Resources.Requests[corev1.ResourceCPU] = cpuQuantity
	}

	if config.Memory != "" {
		memQuantity, err := resource.ParseQuantity(config.Memory)
		if err != nil {
			return fmt.Errorf("invalid memory quantity: %v", err)
		}
		container.Resources.Requests[corev1.ResourceMemory] = memQuantity
	}

	return nil
}

func (r *RollbackManager) SaveToFile(filename string) error {
	r.mu.RLock()
	defer r.mu.RUnlock()

	data, err := json.MarshalIndent(r.history, "", "  ")
	if err != nil {
		return err
	}

	// #nosec G306 - config files need to be readable by the process owner
	return os.WriteFile(filename, data, 0600)
}

func (r *RollbackManager) LoadFromFile(filename string) error {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Sanitize the file path to prevent path traversal
	cleanPath := filepath.Clean(filename)
	// #nosec G304 - filename is provided by the operator, not user input
	data, err := os.ReadFile(cleanPath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}

	return json.Unmarshal(data, &r.history)
}
