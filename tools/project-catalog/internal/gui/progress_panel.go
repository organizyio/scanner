//go:build cgo

package gui

import (
	"fmt"
	"time"

	"scan-projects/internal/controller"

	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

func NewProgressPanel(ctrl *controller.Controller) *widget.Label {
	text := binding.NewString()
	label := widget.NewLabelWithData(text)

	go func() {
		t := time.NewTicker(1 * time.Second)
		defer t.Stop()

		for range t.C {
			snap := ctrl.Progress().Snapshot()
			msg := fmt.Sprintf(
				"%s | files=%d | hashed=%d | %.1f MB | %.1f files/sec",
				snap.State,
				snap.FilesSeen,
				snap.FilesHashed,
				snap.MBProcessed,
				snap.FilesPerSecond,
			)
			_ = text.Set(msg) // thread-safe
		}
	}()

	return label
}
