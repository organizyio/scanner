//go:build cgo

package gui

import (
	"fyne.io/fyne/v2"
	fynetest "fyne.io/fyne/v2/test"
	"fyne.io/fyne/v2/widget"
	"scan-projects/internal/controller"
	"scan-projects/internal/storage"
	"testing"
)

type capturedTrayApp struct {
	fyne.App
	menu         *fyne.Menu
	quits        int
	queuedDriver *queuedTrayDriver
}

func (a *capturedTrayApp) SetSystemTrayMenu(menu *fyne.Menu) { a.menu = menu }
func (a *capturedTrayApp) SetSystemTrayIcon(fyne.Resource)   {}
func (a *capturedTrayApp) SetSystemTrayWindow(fyne.Window)   {}

type queuedTrayDriver struct {
	fyne.Driver
	queue []func()
}

func (d *queuedTrayDriver) DoFromGoroutine(fn func(), wait bool) {
	if wait {
		fn()
		return
	}
	d.queue = append(d.queue, fn)
}
func (a *capturedTrayApp) Driver() fyne.Driver {
	if a.queuedDriver != nil {
		return a.queuedDriver
	}
	return a.App.Driver()
}

func (a *capturedTrayApp) Quit() { a.quits++ }

func TestPublicTrayCannotStartAfterExitRequested(t *testing.T) {
	home := t.TempDir()
	for _, name := range []string{"HOME", "XDG_CONFIG_HOME", "APPDATA", "USERPROFILE"} {
		t.Setenv(name, home)
	}
	base := fynetest.NewApp()
	defer base.Quit()
	app := &capturedTrayApp{App: base}
	window := fynetest.NewWindow(widget.NewLabel("Synthetic"))
	defer window.Close()
	ctrl := controller.New(app)
	profile := ctrl.Profiles().Active()
	profile.ScanRoots = []storage.ScanRoot{{Path: t.TempDir(), Enabled: true}}
	if err := ctrl.Profiles().UpdateActive(profile); err != nil {
		t.Fatal(err)
	}
	initial := ctrl.Progress().State()
	SetupTray(app, window, ctrl)
	var start, exit func()
	for _, item := range app.menu.Items {
		switch item.Label {
		case "Start":
			start = item.Action
		case "Exit":
			exit = item.Action
		}
	}
	if start == nil || exit == nil {
		t.Fatal("missing tray actions")
	}
	driver := &queuedTrayDriver{Driver: base.Driver()}
	app.queuedDriver = driver
	fyne.SetCurrentApp(app)
	defer fyne.SetCurrentApp(base)
	exit()
	start()
	exit()
	if app.quits != 0 {
		t.Fatal("native exit bypassed UI dispatcher")
	}
	// Exit and Start are queued. Exit marks the gate immediately, then its
	// cancellation and terminal-state inspection run in the same queue as Start.
	if len(driver.queue) != 2 {
		t.Fatalf("unexpected queued actions %d", len(driver.queue))
	}
	for len(driver.queue) > 0 {
		action := driver.queue[0]
		driver.queue = driver.queue[1:]
		action()
	}
	if app.quits != 1 {
		t.Fatalf("quit must coalesce, got %d", app.quits)
	}
	if ctrl.IsRunning() || ctrl.Progress().State() != initial {
		t.Fatal("tray started a scan after exit request")
	}
}
