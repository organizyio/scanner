//go:build !cgo

package nativea11y

import (
	"errors"
	"fyne.io/fyne/v2"
)

type Adapter struct{}

func attach(fyne.Window, func(Action)) (*Adapter, error) {
	return nil, errors.New("native accessibility requires a native CGO build")
}
func (*Adapter) Update(Snapshot) error { return errors.New("native accessibility unavailable") }
func (*Adapter) Close()                {}
