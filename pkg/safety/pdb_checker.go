package safety

import (
	"context"
	"fmt"

	appsv1 "k8s.io/api/apps/v1"
	policyv1 "k8s.io/api/policy/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/util/intstr"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
)

type PDBCheckResult struct {
	HasPDB            bool
	PDBName           string
	IsSafe            bool
	CurrentReplicas   int32
	AvailableReplicas int32
	MinAvailable      int32
	MaxUnavailable    int32
	PlannedDisruption int32
	Message           string
}

type PDBChecker struct {
	kubeClient kubernetes.Interface
}

func NewPDBChecker(kubeClient kubernetes.Interface) *PDBChecker {
	return &PDBChecker{
		kubeClient: kubeClient,
	}
}

func (p *PDBChecker) CheckPDBSafety(ctx context.Context, namespace, kind, name string, plannedDisruption int32) (*PDBCheckResult, error) {
	if name == "" {
		pdbList, err := p.kubeClient.PolicyV1().PodDisruptionBudgets(namespace).List(ctx, metav1.ListOptions{})
		if err != nil {
			return nil, fmt.Errorf("failed to list PDBs: %v", err)
		}
		if len(pdbList.Items) == 0 {
			return &PDBCheckResult{
				HasPDB:  false,
				IsSafe:  true,
				Message: "No PDBs found in namespace",
			}, nil
		}
		return &PDBCheckResult{
			HasPDB:  true,
			IsSafe:  true,
			Message: fmt.Sprintf("Found %d PDB(s) in namespace, per-workload validation will occur during apply", len(pdbList.Items)),
		}, nil
	}

	workloadLabels, currentReplicas, availableReplicas, err := p.getWorkloadInfo(ctx, namespace, kind, name)
	if err != nil {
		return nil, fmt.Errorf("failed to get workload info: %v", err)
	}

	pdb, err := p.findMatchingPDB(ctx, namespace, workloadLabels)
	if err != nil {
		return nil, fmt.Errorf("failed to find PDB: %v", err)
	}

	if pdb == nil {
		return &PDBCheckResult{
			HasPDB:            false,
			CurrentReplicas:   currentReplicas,
			AvailableReplicas: availableReplicas,
			IsSafe:            true,
			Message:           "No PDB found, update is safe",
		}, nil
	}

	result := &PDBCheckResult{
		HasPDB:            true,
		PDBName:           pdb.Name,
		CurrentReplicas:   currentReplicas,
		AvailableReplicas: availableReplicas,
		PlannedDisruption: plannedDisruption,
	}

	if pdb.Spec.MinAvailable != nil {
		minAvail := p.calculateIntOrPercent(pdb.Spec.MinAvailable, currentReplicas)
		result.MinAvailable = minAvail
		afterDisruption := availableReplicas - plannedDisruption
		result.IsSafe = afterDisruption >= minAvail
		result.Message = fmt.Sprintf("PDB requires minAvailable=%d, after disruption will have %d available",
			minAvail, afterDisruption)
	} else if pdb.Spec.MaxUnavailable != nil {
		maxUnavail := p.calculateIntOrPercent(pdb.Spec.MaxUnavailable, currentReplicas)
		result.MaxUnavailable = maxUnavail
		currentUnavailable := currentReplicas - availableReplicas
		afterDisruption := currentUnavailable + plannedDisruption
		result.IsSafe = afterDisruption <= maxUnavail
		result.Message = fmt.Sprintf("PDB allows maxUnavailable=%d, after disruption will have %d unavailable",
			maxUnavail, afterDisruption)
	}

	if !result.IsSafe {
		klog.V(3).Infof("PDB violation detected for %s/%s: %s", namespace, name, result.Message)
	}

	return result, nil
}

func (p *PDBChecker) getWorkloadInfo(ctx context.Context, namespace, kind, name string) (map[string]string, int32, int32, error) {
	switch kind {
	case "Deployment":
		deploy, err := p.kubeClient.AppsV1().Deployments(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, 0, 0, err
		}
		replicas := int32(1)
		if deploy.Spec.Replicas != nil {
			replicas = *deploy.Spec.Replicas
		}
		return deploy.Spec.Selector.MatchLabels, replicas, deploy.Status.AvailableReplicas, nil

	case "StatefulSet":
		sts, err := p.kubeClient.AppsV1().StatefulSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, 0, 0, err
		}
		replicas := int32(1)
		if sts.Spec.Replicas != nil {
			replicas = *sts.Spec.Replicas
		}
		return sts.Spec.Selector.MatchLabels, replicas, sts.Status.AvailableReplicas, nil

	case "DaemonSet":
		ds, err := p.kubeClient.AppsV1().DaemonSets(namespace).Get(ctx, name, metav1.GetOptions{})
		if err != nil {
			return nil, 0, 0, err
		}
		return ds.Spec.Selector.MatchLabels, ds.Status.DesiredNumberScheduled, ds.Status.NumberAvailable, nil

	default:
		return nil, 0, 0, fmt.Errorf("unsupported workload kind: %s", kind)
	}
}

func (p *PDBChecker) findMatchingPDB(ctx context.Context, namespace string, workloadLabels map[string]string) (*policyv1.PodDisruptionBudget, error) {
	pdbList, err := p.kubeClient.PolicyV1().PodDisruptionBudgets(namespace).List(ctx, metav1.ListOptions{})
	if err != nil {
		return nil, err
	}

	for _, pdb := range pdbList.Items {
		if pdb.Spec.Selector == nil {
			continue
		}

		selector, err := metav1.LabelSelectorAsSelector(pdb.Spec.Selector)
		if err != nil {
			continue
		}

		if selector.Matches(labels.Set(workloadLabels)) {
			return &pdb, nil
		}
	}

	return nil, nil
}

func (p *PDBChecker) calculateIntOrPercent(value *intstr.IntOrString, total int32) int32 {
	if value.Type == intstr.Int {
		return int32(value.IntVal)
	}

	percent, _ := intstr.GetScaledValueFromIntOrPercent(value, int(total), true)
	return int32(percent)
}

func (p *PDBChecker) CalculateSafeDisruptionBudget(ctx context.Context, namespace, kind, name string) (int32, error) {
	result, err := p.CheckPDBSafety(ctx, namespace, kind, name, 0)
	if err != nil {
		return 0, err
	}

	if !result.HasPDB {
		return result.CurrentReplicas, nil
	}

	if result.MinAvailable > 0 {
		return result.AvailableReplicas - result.MinAvailable, nil
	}

	if result.MaxUnavailable > 0 {
		currentUnavailable := result.CurrentReplicas - result.AvailableReplicas
		return result.MaxUnavailable - currentUnavailable, nil
	}

	return 0, nil
}

func createTestDeployment(name, namespace string, replicas, availableReplicas int32, labels map[string]string) *appsv1.Deployment {
	return &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{
				MatchLabels: labels,
			},
		},
		Status: appsv1.DeploymentStatus{
			Replicas:          replicas,
			AvailableReplicas: availableReplicas,
		},
	}
}
