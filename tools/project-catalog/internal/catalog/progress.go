package catalog

import (
	"fmt"
	"math"
	"os"
	"strings"
	"time"
)

// ProgressBar displays scanning progress with a visual bar, percentage,
// and statistics about directories scanned and projects found.
type ProgressBar struct {
	Total     int
	Current   int
	Projects  int
	StartTime time.Time
	Width     int
}

func getTerminalWidth() int {
	if cols := os.Getenv("COLUMNS"); cols != "" {
		var width int
		if _, err := fmt.Sscanf(cols, "%d", &width); err == nil && width > 20 {
			return width - 20
		}
	}
	return 50
}

// NewProgressBar creates a new progress bar with the specified total count.
func NewProgressBar(total int) *ProgressBar {
	width := getTerminalWidth()
	if width < 20 {
		width = 20
	}
	if width > 80 {
		width = 80
	}
	return &ProgressBar{
		Total:     total,
		Current:   0,
		Projects:  0,
		StartTime: time.Now(),
		Width:     width,
	}
}

// Increment advances the progress bar by one directory, optionally marking
// it as a project. It then re-renders the progress display.
func (p *ProgressBar) Increment(isProject bool) {
	p.Current++
	if isProject {
		p.Projects++
	}
	p.Render()
}

func formatDuration(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.1fs", d.Seconds())
	}
	minutes := int(d.Minutes())
	seconds := int(d.Seconds()) % 60
	return fmt.Sprintf("%dm%ds", minutes, seconds)
}

// Render updates the progress bar display on stdout.
func (p *ProgressBar) Render() {
	if p.Total == 0 {
		return
	}

	ratio := float64(p.Current) / float64(p.Total)
	if ratio > 1 {
		ratio = 1
	}
	filled := int(math.Round(ratio * float64(p.Width)))
	bar := fmt.Sprintf("[%s%s]",
		strings.Repeat("=", filled),
		strings.Repeat("-", p.Width-filled),
	)

	percent := int(ratio * 100)

	elapsed := time.Since(p.StartTime)
	var etaStr string
	if p.Current > 0 && p.Current < p.Total {
		avgTimePerDir := elapsed / time.Duration(p.Current)
		remaining := p.Total - p.Current
		eta := avgTimePerDir * time.Duration(remaining)
		etaStr = fmt.Sprintf(" | ETA: %s", formatDuration(eta))
	}

	fmt.Fprintf(os.Stdout,
		"\r%s %3d%% | scanned: %d/%d dirs | projects: %d%s",
		bar, percent, p.Current, p.Total, p.Projects, etaStr,
	)

	if p.Current >= p.Total {
		totalTime := time.Since(p.StartTime)
		fmt.Fprintf(os.Stdout, "  (completed in %s)\n", formatDuration(totalTime))
	}
}
