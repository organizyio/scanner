//go:build cgo

package gui

import (
	"context"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
	"time"

	"scan-projects/internal/controller"

	"fyne.io/fyne/v2"
	"fyne.io/fyne/v2/widget"
)

func NewFolderTree(ctx context.Context, ctrl *controller.Controller, details *widget.Label) *FolderTree {
	const rootID = ""

	tree := widget.NewTree(
		func(uid widget.TreeNodeID) []widget.TreeNodeID {
			return treeChildren(ctrl, uid)
		},
		func(uid widget.TreeNodeID) bool {
			return len(treeChildren(ctrl, uid)) > 0
		},
		func(branch bool) fyne.CanvasObject {
			return widget.NewLabel("")
		},
		func(uid widget.TreeNodeID, branch bool, obj fyne.CanvasObject) {
			obj.(*widget.Label).SetText(nodeLabel(ctrl, uid))
		},
	)

	tree.Root = rootID

	tree.OnSelected = func(id widget.TreeNodeID) {
		if id == rootID {
			return
		}
		showFolderDetails(ctrl, id, details)
	}

	startUIPoller(ctx, 2*time.Second, fyne.Do, tree.Refresh)

	return newFolderTree(tree)
}

func treeChildren(ctrl *controller.Controller, uid string) []string {
	if uid == "" {
		p := ctrl.Profiles().Active()
		out := make([]string, 0, len(p.ScanRoots))
		for _, r := range p.ScanRoots {
			out = append(out, r.Path)
		}
		return out
	}

	path := uid
	folders := ctrl.SnapshotFolders()
	if folders == nil {
		return nil
	}

	prefix := ensureTrailingSep(path)
	var kids []string

	for fpath := range folders {
		if strings.HasPrefix(fpath, prefix) {
			rest := strings.TrimPrefix(fpath, prefix)
			if rest != "" && !strings.Contains(rest, string(filepath.Separator)) {
				kids = append(kids, fpath)
			}
		}
	}

	sort.Strings(kids)
	return kids
}

func nodeLabel(ctrl *controller.Controller, uid string) string {
	for _, root := range ctrl.Profiles().Active().ScanRoots {
		if root.Path == uid {
			if root.Enabled {
				return uid + " (enabled)"
			}
			return uid + " (disabled)"
		}
	}

	folders := ctrl.SnapshotFolders()
	if fr, ok := folders[uid]; ok && fr != nil {
		name := filepath.Base(uid)
		flags := ""
		if fr.IsProjectRoot {
			flags = " [PROJECT:" + fr.ProjectKind + "]"
		}
		return fmt.Sprintf(
			"%s (files:%d H:%.2f)%s",
			name,
			fr.FileCount,
			fr.ExtensionEntropy,
			flags,
		)
	}
	return uid
}

func showFolderDetails(ctrl *controller.Controller, path string, details *widget.Label) {

	folders := ctrl.SnapshotFolders()
	fr := folders[path]
	if fr == nil {
		details.SetText(path + "\n\n(no data yet)")
		return
	}

	details.SetText(fmt.Sprintf(
		"%s\n\nFiles: %d\nSubfolders: %d\nSize: %.2f MB\nEntropy: %.4f\nProject: %v (%s)",
		fr.Path,
		fr.FileCount,
		fr.SubfolderCount,
		float64(fr.TotalSizeBytes)/(1024*1024),
		fr.ExtensionEntropy,
		fr.IsProjectRoot,
		fr.ProjectKind,
	))
}

func ensureTrailingSep(p string) string {
	if strings.HasSuffix(p, string(filepath.Separator)) {
		return p
	}
	return p + string(filepath.Separator)
}
