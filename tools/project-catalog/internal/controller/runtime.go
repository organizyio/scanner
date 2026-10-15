package controller

import "scan-projects/internal/scan"

type Runtime struct {
	Opts *scan.RuntimeOptions
}

func NewRuntime() *Runtime {
	return &Runtime{Opts: scan.NewRuntimeOptions()}
}
