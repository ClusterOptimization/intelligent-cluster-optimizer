package safety

import (
	"time"

	optimizerv1alpha1 "intelligent-cluster-optimizer/pkg/apis/optimizer/v1alpha1"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/klog/v2"
)

type CircuitBreaker struct{}

func NewCircuitBreaker() *CircuitBreaker {
	return &CircuitBreaker{}
}

func (cb *CircuitBreaker) ShouldAllow(config *optimizerv1alpha1.OptimizerConfig) bool {
	if config.Spec.CircuitBreaker == nil || !config.Spec.CircuitBreaker.Enabled {
		return true
	}

	if config.Status.CircuitState == optimizerv1alpha1.CircuitStateOpen {
		timeout, err := time.ParseDuration(config.Spec.CircuitBreaker.Timeout)
		if err != nil {
			timeout = 5 * time.Minute
		}

		if config.Status.LastUpdateTime != nil {
			elapsed := time.Since(config.Status.LastUpdateTime.Time)
			if elapsed >= timeout {
				config.Status.CircuitState = optimizerv1alpha1.CircuitStateHalfOpen
				config.Status.ConsecutiveErrors = 0
				config.Status.ConsecutiveSuccesses = 0
				klog.Infof("Circuit breaker entering half-open state for %s/%s after %v timeout", config.Namespace, config.Name, timeout)
				return true
			}
		}
		return false
	}

	return true
}

func (cb *CircuitBreaker) RecordSuccess(config *optimizerv1alpha1.OptimizerConfig) (stateChanged bool) {
	if config.Spec.CircuitBreaker == nil || !config.Spec.CircuitBreaker.Enabled {
		return false
	}

	config.Status.ConsecutiveErrors = 0
	config.Status.ConsecutiveSuccesses++
	config.Status.TotalUpdatesApplied++

	previousState := config.Status.CircuitState

	if config.Status.CircuitState == optimizerv1alpha1.CircuitStateHalfOpen {
		successThreshold := config.Spec.CircuitBreaker.SuccessThreshold
		if successThreshold == 0 {
			successThreshold = 3
		}

		if config.Status.ConsecutiveSuccesses >= successThreshold {
			config.Status.CircuitState = optimizerv1alpha1.CircuitStateClosed
			config.Status.ConsecutiveSuccesses = 0
			klog.Infof("Circuit breaker closed for %s/%s after %d consecutive successes",
				config.Namespace, config.Name, successThreshold)
			return true
		}
	}

	now := metav1.NewTime(time.Now())
	config.Status.LastUpdateTime = &now

	return config.Status.CircuitState != previousState
}

func (cb *CircuitBreaker) RecordFailure(config *optimizerv1alpha1.OptimizerConfig, err error) (stateChanged bool) {
	if config.Spec.CircuitBreaker == nil || !config.Spec.CircuitBreaker.Enabled {
		return false
	}

	config.Status.ConsecutiveSuccesses = 0
	config.Status.ConsecutiveErrors++
	config.Status.TotalUpdatesFailed++

	previousState := config.Status.CircuitState

	errorThreshold := config.Spec.CircuitBreaker.ErrorThreshold
	if errorThreshold == 0 {
		errorThreshold = 5
	}

	if config.Status.ConsecutiveErrors >= errorThreshold {
		if config.Status.CircuitState != optimizerv1alpha1.CircuitStateOpen {
			config.Status.CircuitState = optimizerv1alpha1.CircuitStateOpen
			klog.Warningf("Circuit breaker opened for %s/%s after %d consecutive errors: %v",
				config.Namespace, config.Name, config.Status.ConsecutiveErrors, err)
			return true
		}
	}

	now := metav1.NewTime(time.Now())
	config.Status.LastUpdateTime = &now

	return config.Status.CircuitState != previousState
}

func (cb *CircuitBreaker) GetStateName(state optimizerv1alpha1.CircuitState) string {
	switch state {
	case optimizerv1alpha1.CircuitStateClosed:
		return "Closed"
	case optimizerv1alpha1.CircuitStateOpen:
		return "Open"
	case optimizerv1alpha1.CircuitStateHalfOpen:
		return "HalfOpen"
	default:
		return "Unknown"
	}
}
