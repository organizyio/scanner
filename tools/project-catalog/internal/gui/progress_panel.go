//go:build cgo

package gui

import (
	"context"
	"fmt"
	"fyne.io/fyne/v2"
	"time"

	"scan-projects/internal/controller"

	"fyne.io/fyne/v2/data/binding"
	"fyne.io/fyne/v2/widget"
)

func NewProgressPanel(ctx context.Context, ctrl *controller.Controller) *widget.Label {
	text := binding.NewString()
	label := widget.NewLabelWithData(text)
	label.Wrapping = fyne.TextWrapWord

	startUIPoller(ctx, time.Second, fyne.Do, func() {
		snap := ctrl.Progress().Snapshot()
		msg := fmt.Sprintf(
			"%s | files=%d | hashed=%d | %.1f MB | %.1f files/sec",
			snap.State,
			snap.FilesSeen,
			snap.FilesHashed,
			snap.MBProcessed,
			snap.FilesPerSecond,
		)
		if ctrl.ResumeFrom() != "" {
			msg += " | Resumed partial inventory (inclusive checkpoint cursor)"
		}
		if saveErr := ctrl.Profiles().LastError(); saveErr != nil {
			msg += " | Settings not saved: " + saveErr.Error()
		}
		if warning := ctrl.LastError(); warning != "" {
			msg += " | " + warning
		}
		_ = text.Set(msg) // UI lifecycle dispatcher
	})

	return label
}
