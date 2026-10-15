package scan

import "sync/atomic"

type RuntimeOptions struct {
	HashFiles        atomic.Bool
	HashMaxSizeBytes atomic.Int64
	ProjectDetection atomic.Bool
}

func NewRuntimeOptions() *RuntimeOptions {
	r := &RuntimeOptions{}
	r.HashFiles.Store(true)
	r.HashMaxSizeBytes.Store(200 * 1024 * 1024)
	r.ProjectDetection.Store(true)
	return r
}
