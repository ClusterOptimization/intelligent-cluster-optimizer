package main

import (
	"context"
	"flag"
	"os"
	"path/filepath"
	"time"

	"intelligent-cluster-optimizer/pkg/apis/optimizer/v1alpha1"
	"intelligent-cluster-optimizer/pkg/controller"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	"k8s.io/klog/v2"
)

var (
	kubeconfig    string
	namespace     string
	workers       int
	leaseLockName string
	leaseLockNS   string
	leaderElect   bool
	leaseDuration time.Duration
	renewDeadline time.Duration
	retryPeriod   time.Duration
)

func main() {
	klog.InitFlags(nil)
	flag.StringVar(&kubeconfig, "kubeconfig", filepath.Join(os.Getenv("HOME"), ".kube", "config"), "Path to kubeconfig")
	flag.StringVar(&namespace, "namespace", "default", "Namespace to watch OptimizerConfigs")
	flag.IntVar(&workers, "workers", 2, "Number of worker threads")
	flag.StringVar(&leaseLockName, "lease-lock-name", "optimizer-controller", "Name of lease lock")
	flag.StringVar(&leaseLockNS, "lease-lock-namespace", "default", "Namespace for lease lock")
	flag.BoolVar(&leaderElect, "leader-elect", true, "Enable leader election")
	flag.DurationVar(&leaseDuration, "lease-duration", 15*time.Second, "Lease duration")
	flag.DurationVar(&renewDeadline, "renew-deadline", 10*time.Second, "Renew deadline")
	flag.DurationVar(&retryPeriod, "retry-period", 2*time.Second, "Retry period")
	flag.Parse()

	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		klog.Fatalf("Failed to build config: %v", err)
	}

	kubeClient, err := kubernetes.NewForConfig(config)
	if err != nil {
		klog.Fatalf("Failed to create kubernetes client: %v", err)
	}

	optimizerClient, err := v1alpha1.NewOptimizerConfigClient(config, namespace)
	if err != nil {
		klog.Fatalf("Failed to create optimizer client: %v", err)
	}

	reconciler := controller.NewReconciler()
	ctrl := controller.NewOptimizerController(kubeClient, optimizerClient, reconciler, namespace)

	ctx := context.Background()

	if !leaderElect {
		klog.Info("Running without leader election")
		if err := ctrl.Run(ctx, workers); err != nil {
			klog.Fatalf("Error running controller: %v", err)
		}
		return
	}

	id, err := os.Hostname()
	if err != nil {
		klog.Fatalf("Failed to get hostname: %v", err)
	}

	lock := &resourcelock.LeaseLock{
		LeaseMeta: metav1.ObjectMeta{
			Name:      leaseLockName,
			Namespace: leaseLockNS,
		},
		Client: kubeClient.CoordinationV1(),
		LockConfig: resourcelock.ResourceLockConfig{
			Identity: id,
		},
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:            lock,
		ReleaseOnCancel: true,
		LeaseDuration:   leaseDuration,
		RenewDeadline:   renewDeadline,
		RetryPeriod:     retryPeriod,
		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.Infof("Started leading as %s", id)
				if err := ctrl.Run(ctx, workers); err != nil {
					klog.Fatalf("Error running controller: %v", err)
				}
			},
			OnStoppedLeading: func() {
				klog.Infof("Leader lost: %s", id)
				os.Exit(0)
			},
			OnNewLeader: func(identity string) {
				if identity == id {
					return
				}
				klog.Infof("New leader elected: %s", identity)
			},
		},
	})
}
