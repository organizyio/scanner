//go:build cgo

package main

import (
	"scan-projects/internal/controller"
	"scan-projects/internal/gui"
)

func main() {
	a := gui.NewApp()
	ctrl := controller.New(a)

	gui.Run(a, ctrl)
}
