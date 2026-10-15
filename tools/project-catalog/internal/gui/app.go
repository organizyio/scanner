//go:build cgo

package gui

import (
	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/app"
)

func NewApp() fyne.App {
	return app.New()
}
