//go:build !cgo

package gui

// NeedsCGO documents that the real Fyne UI is only built with CGO (OpenGL) enabled.
const NeedsCGO = true
