package scheduler

import (
	"fmt"
	"time"

	optimizerv1alpha1 "intelligent-cluster-optimizer/pkg/apis/optimizer/v1alpha1"

	"github.com/robfig/cron/v3"
	"k8s.io/klog/v2"
)

type MaintenanceWindowChecker struct {
	parser cron.Parser
}

func NewMaintenanceWindowChecker() *MaintenanceWindowChecker {
	return &MaintenanceWindowChecker{
		parser: cron.NewParser(cron.Minute | cron.Hour | cron.Dom | cron.Month | cron.Dow),
	}
}

func (m *MaintenanceWindowChecker) IsInMaintenanceWindow(config *optimizerv1alpha1.OptimizerConfig) bool {
	if len(config.Spec.MaintenanceWindows) == 0 {
		return true
	}

	now := time.Now()
	for _, window := range config.Spec.MaintenanceWindows {
		if m.isInWindow(window, now) {
			return true
		}
	}

	return false
}

func (m *MaintenanceWindowChecker) GetNextMaintenanceWindow(config *optimizerv1alpha1.OptimizerConfig) *time.Time {
	if len(config.Spec.MaintenanceWindows) == 0 {
		return nil
	}

	var nextWindow *time.Time
	now := time.Now()

	for _, window := range config.Spec.MaintenanceWindows {
		next := m.getNextWindowStart(window, now)
		if next != nil && (nextWindow == nil || next.Before(*nextWindow)) {
			nextWindow = next
		}
	}

	return nextWindow
}

func (m *MaintenanceWindowChecker) isInWindow(window optimizerv1alpha1.MaintenanceWindow, now time.Time) bool {
	location, err := m.getLocation(window.Timezone)
	if err != nil {
		klog.Warningf("Invalid timezone %s, using UTC: %v", window.Timezone, err)
		location = time.UTC
	}

	nowInTz := now.In(location)

	schedule, err := m.parser.Parse(window.Schedule)
	if err != nil {
		klog.Warningf("Invalid cron schedule %s: %v", window.Schedule, err)
		return false
	}

	duration, err := time.ParseDuration(window.Duration)
	if err != nil {
		klog.Warningf("Invalid duration %s: %v", window.Duration, err)
		return false
	}

	lastStart := schedule.Next(nowInTz.Add(-duration - time.Minute))
	for lastStart.Before(nowInTz) {
		windowEnd := lastStart.Add(duration)
		if nowInTz.After(lastStart) && nowInTz.Before(windowEnd) {
			klog.V(4).Infof("Currently in maintenance window: %s - %s", lastStart.Format(time.RFC3339), windowEnd.Format(time.RFC3339))
			return true
		}
		lastStart = schedule.Next(lastStart)
	}

	return false
}

func (m *MaintenanceWindowChecker) getNextWindowStart(window optimizerv1alpha1.MaintenanceWindow, now time.Time) *time.Time {
	location, err := m.getLocation(window.Timezone)
	if err != nil {
		klog.Warningf("Invalid timezone %s, using UTC: %v", window.Timezone, err)
		location = time.UTC
	}

	nowInTz := now.In(location)

	schedule, err := m.parser.Parse(window.Schedule)
	if err != nil {
		klog.Warningf("Invalid cron schedule %s: %v", window.Schedule, err)
		return nil
	}

	next := schedule.Next(nowInTz)
	return &next
}

func (m *MaintenanceWindowChecker) getLocation(timezone string) (*time.Location, error) {
	if timezone == "" {
		return time.UTC, nil
	}
	return time.LoadLocation(timezone)
}

func (m *MaintenanceWindowChecker) ValidateMaintenanceWindow(window optimizerv1alpha1.MaintenanceWindow) error {
	if _, err := m.parser.Parse(window.Schedule); err != nil {
		return fmt.Errorf("invalid cron schedule %s: %v", window.Schedule, err)
	}

	if _, err := time.ParseDuration(window.Duration); err != nil {
		return fmt.Errorf("invalid duration %s: %v", window.Duration, err)
	}

	if window.Timezone != "" {
		if _, err := time.LoadLocation(window.Timezone); err != nil {
			return fmt.Errorf("invalid timezone %s: %v", window.Timezone, err)
		}
	}

	return nil
}
