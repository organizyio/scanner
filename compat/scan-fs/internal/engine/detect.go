package engine

import (
	"fmt"
	"runtime"
)

// BinaryName returns the bundled scan-engine filename for the current OS/arch.
func BinaryName() (string, error) {
	switch {
	case runtime.GOOS == "linux" && runtime.GOARCH == "amd64":
		return "scan-linux-amd64", nil
	case runtime.GOOS == "darwin" && runtime.GOARCH == "amd64":
		return "", fmt.Errorf("darwin/amd64: scan-engine releases only publish an aarch64 macOS binary; use Apple Silicon, set SCAN_ENGINE_PATH, or build scan from source")
	case runtime.GOOS == "darwin" && runtime.GOARCH == "arm64":
		return "scan-darwin-arm64", nil
	case runtime.GOOS == "windows" && runtime.GOARCH == "amd64":
		return "scan-windows-amd64.exe", nil
	default:
		return "", fmt.Errorf("unsupported platform %s/%s", runtime.GOOS, runtime.GOARCH)
	}
}
