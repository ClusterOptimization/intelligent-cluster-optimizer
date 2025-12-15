package applier

import (
	"context"
	"encoding/json"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/resource"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type Applier struct {
	kubeClient kubernetes.Interface
}

func NewApplier(kubeClient kubernetes.Interface) *Applier {
	return &Applier{
		kubeClient: kubeClient,
	}
}

func (a *Applier) Apply(ctx context.Context, recommendation *ResourceRecommendation, dryRun bool) (*ApplyResult, error) {
	if dryRun {
		return a.DryRunApply(ctx, recommendation)
	}
	return a.LiveApply(ctx, recommendation)
}

func (a *Applier) DryRunApply(ctx context.Context, recommendation *ResourceRecommendation) (*ApplyResult, error) {
	result := &ApplyResult{
		Applied:      false,
		DryRun:       true,
		WorkloadKind: recommendation.WorkloadKind,
		WorkloadName: recommendation.WorkloadName,
		Namespace:    recommendation.Namespace,
		Changes:      []string{},
	}

	if !recommendation.HasChanges() {
		klog.V(3).Infof("[DRY-RUN] No changes needed for %s/%s/%s",
			recommendation.Namespace, recommendation.WorkloadKind, recommendation.WorkloadName)
		return result, nil
	}

	if recommendation.CurrentCPU != recommendation.RecommendedCPU {
		change := fmt.Sprintf("CPU: %s -> %s", recommendation.CurrentCPU, recommendation.RecommendedCPU)
		result.Changes = append(result.Changes, change)
		klog.Infof("[DRY-RUN] Would change %s/%s/%s container=%s: %s",
			recommendation.Namespace, recommendation.WorkloadKind, recommendation.WorkloadName,
			recommendation.ContainerName, change)
	}

	if recommendation.CurrentMemory != recommendation.RecommendedMemory {
		change := fmt.Sprintf("Memory: %s -> %s", recommendation.CurrentMemory, recommendation.RecommendedMemory)
		result.Changes = append(result.Changes, change)
		klog.Infof("[DRY-RUN] Would change %s/%s/%s container=%s: %s",
			recommendation.Namespace, recommendation.WorkloadKind, recommendation.WorkloadName,
			recommendation.ContainerName, change)
	}

	klog.V(2).Infof("[DRY-RUN] Total changes for %s/%s: %d",
		recommendation.WorkloadKind, recommendation.WorkloadName, len(result.Changes))

	return result, nil
}

func (a *Applier) LiveApply(ctx context.Context, recommendation *ResourceRecommendation) (*ApplyResult, error) {
	result := &ApplyResult{
		Applied:      false,
		DryRun:       false,
		WorkloadKind: recommendation.WorkloadKind,
		WorkloadName: recommendation.WorkloadName,
		Namespace:    recommendation.Namespace,
		Changes:      []string{},
	}

	if !recommendation.HasChanges() {
		klog.V(3).Infof("[LIVE] No changes needed for %s/%s/%s",
			recommendation.Namespace, recommendation.WorkloadKind, recommendation.WorkloadName)
		return result, nil
	}

	klog.Infof("[LIVE] Applying changes to %s/%s/%s",
		recommendation.Namespace, recommendation.WorkloadKind, recommendation.WorkloadName)

	switch recommendation.WorkloadKind {
	case "Deployment":
		return a.applyToDeployment(ctx, recommendation, result)
	case "StatefulSet":
		return a.applyToStatefulSet(ctx, recommendation, result)
	case "DaemonSet":
		return a.applyToDaemonSet(ctx, recommendation, result)
	default:
		result.Error = fmt.Errorf("unsupported workload kind: %s", recommendation.WorkloadKind)
		return result, result.Error
	}
}

func (a *Applier) applyToDeployment(ctx context.Context, rec *ResourceRecommendation, result *ApplyResult) (*ApplyResult, error) {
	deploy, err := a.kubeClient.AppsV1().Deployments(rec.Namespace).Get(ctx, rec.WorkloadName, metav1.GetOptions{})
	if err != nil {
		result.Error = fmt.Errorf("failed to get deployment: %v", err)
		return result, result.Error
	}

	patched, changes := a.patchContainerResources(deploy.Spec.Template.Spec.Containers, rec)
	if !patched {
		return result, nil
	}

	result.Changes = changes

	patchData, err := a.createResourcePatch(rec)
	if err != nil {
		result.Error = fmt.Errorf("failed to create patch: %v", err)
		return result, result.Error
	}

	_, err = a.kubeClient.AppsV1().Deployments(rec.Namespace).Patch(
		ctx, rec.WorkloadName, types.StrategicMergePatchType, patchData, metav1.PatchOptions{})
	if err != nil {
		result.Error = fmt.Errorf("failed to patch deployment: %v", err)
		return result, result.Error
	}

	result.Applied = true
	klog.Infof("[LIVE] Successfully applied changes to Deployment %s/%s", rec.Namespace, rec.WorkloadName)
	return result, nil
}

func (a *Applier) applyToStatefulSet(ctx context.Context, rec *ResourceRecommendation, result *ApplyResult) (*ApplyResult, error) {
	sts, err := a.kubeClient.AppsV1().StatefulSets(rec.Namespace).Get(ctx, rec.WorkloadName, metav1.GetOptions{})
	if err != nil {
		result.Error = fmt.Errorf("failed to get statefulset: %v", err)
		return result, result.Error
	}

	patched, changes := a.patchContainerResources(sts.Spec.Template.Spec.Containers, rec)
	if !patched {
		return result, nil
	}

	result.Changes = changes

	patchData, err := a.createResourcePatch(rec)
	if err != nil {
		result.Error = fmt.Errorf("failed to create patch: %v", err)
		return result, result.Error
	}

	_, err = a.kubeClient.AppsV1().StatefulSets(rec.Namespace).Patch(
		ctx, rec.WorkloadName, types.StrategicMergePatchType, patchData, metav1.PatchOptions{})
	if err != nil {
		result.Error = fmt.Errorf("failed to patch statefulset: %v", err)
		return result, result.Error
	}

	result.Applied = true
	klog.Infof("[LIVE] Successfully applied changes to StatefulSet %s/%s", rec.Namespace, rec.WorkloadName)
	return result, nil
}

func (a *Applier) applyToDaemonSet(ctx context.Context, rec *ResourceRecommendation, result *ApplyResult) (*ApplyResult, error) {
	ds, err := a.kubeClient.AppsV1().DaemonSets(rec.Namespace).Get(ctx, rec.WorkloadName, metav1.GetOptions{})
	if err != nil {
		result.Error = fmt.Errorf("failed to get daemonset: %v", err)
		return result, result.Error
	}

	patched, changes := a.patchContainerResources(ds.Spec.Template.Spec.Containers, rec)
	if !patched {
		return result, nil
	}

	result.Changes = changes

	patchData, err := a.createResourcePatch(rec)
	if err != nil {
		result.Error = fmt.Errorf("failed to create patch: %v", err)
		return result, result.Error
	}

	_, err = a.kubeClient.AppsV1().DaemonSets(rec.Namespace).Patch(
		ctx, rec.WorkloadName, types.StrategicMergePatchType, patchData, metav1.PatchOptions{})
	if err != nil {
		result.Error = fmt.Errorf("failed to patch daemonset: %v", err)
		return result, result.Error
	}

	result.Applied = true
	klog.Infof("[LIVE] Successfully applied changes to DaemonSet %s/%s", rec.Namespace, rec.WorkloadName)
	return result, nil
}

func (a *Applier) patchContainerResources(containers []corev1.Container, rec *ResourceRecommendation) (bool, []string) {
	changes := []string{}

	for i := range containers {
		if containers[i].Name == rec.ContainerName {
			if rec.RecommendedCPU != "" && rec.CurrentCPU != rec.RecommendedCPU {
				changes = append(changes, fmt.Sprintf("CPU: %s -> %s", rec.CurrentCPU, rec.RecommendedCPU))
			}
			if rec.RecommendedMemory != "" && rec.CurrentMemory != rec.RecommendedMemory {
				changes = append(changes, fmt.Sprintf("Memory: %s -> %s", rec.CurrentMemory, rec.RecommendedMemory))
			}
			return len(changes) > 0, changes
		}
	}

	return false, changes
}

func (a *Applier) createResourcePatch(rec *ResourceRecommendation) ([]byte, error) {
	patch := map[string]interface{}{
		"spec": map[string]interface{}{
			"template": map[string]interface{}{
				"spec": map[string]interface{}{
					"containers": []map[string]interface{}{
						{
							"name": rec.ContainerName,
							"resources": map[string]interface{}{
								"requests": map[string]string{},
							},
						},
					},
				},
			},
		},
	}

	requests := patch["spec"].(map[string]interface{})["template"].(map[string]interface{})["spec"].(map[string]interface{})["containers"].([]map[string]interface{})[0]["resources"].(map[string]interface{})["requests"].(map[string]string)

	if rec.RecommendedCPU != "" {
		requests["cpu"] = rec.RecommendedCPU
	}
	if rec.RecommendedMemory != "" {
		requests["memory"] = rec.RecommendedMemory
	}

	return json.Marshal(patch)
}

func (a *Applier) GetCurrentResources(ctx context.Context, namespace, kind, name, containerName string) (*ResourceRecommendation, error) {
	rec := &ResourceRecommendation{
		Namespace:     namespace,
		WorkloadKind:  kind,
		WorkloadName:  name,
		ContainerName: containerName,
	}

	var containers []corev1.Container
	var err error

	switch kind {
	case "Deployment":
		var deploy *appsv1.Deployment
		deploy, err = a.kubeClient.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			containers = deploy.Spec.Template.Spec.Containers
		}
	case "StatefulSet":
		var sts *appsv1.StatefulSet
		sts, err = a.kubeClient.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			containers = sts.Spec.Template.Spec.Containers
		}
	case "DaemonSet":
		var ds *appsv1.DaemonSet
		ds, err = a.kubeClient.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err == nil {
			containers = ds.Spec.Template.Spec.Containers
		}
	default:
		return nil, fmt.Errorf("unsupported kind: %s", kind)
	}

	if err != nil {
		return nil, err
	}

	for _, container := range containers {
		if container.Name == containerName {
			if cpu, ok := container.Resources.Requests[corev1.ResourceCPU]; ok {
				rec.CurrentCPU = cpu.String()
			}
			if mem, ok := container.Resources.Requests[corev1.ResourceMemory]; ok {
				rec.CurrentMemory = mem.String()
			}
			return rec, nil
		}
	}

	return rec, nil
}

func ParseResourceQuantity(value string) (resource.Quantity, error) {
	return resource.ParseQuantity(value)
}
