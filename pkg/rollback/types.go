package rollback

import (
	"time"
)

type WorkloadConfig struct {
	Namespace     string
	Kind          string
	Name          string
	ContainerName string
	CPU           string
	Memory        string
	Timestamp     time.Time
}

func (w *WorkloadConfig) Key() string {
	return w.Namespace + "/" + w.Kind + "/" + w.Name + "/" + w.ContainerName
}
