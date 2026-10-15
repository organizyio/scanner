package scan

import (
	"sync/atomic"
	"time"
)

type Progress struct {
	startTime time.Time

	filesSeen      atomic.Int64
	filesHashed    atomic.Int64
	bytesProcessed atomic.Int64

	lastPath atomic.Value // string
	state    atomic.Value // idle | running | paused | canceled | finished
}

func NewProgress() *Progress {
	p := &Progress{startTime: time.Now()}
	p.lastPath.Store("")
	p.state.Store("idle")
	return p
}

func (p *Progress) Reset() {
	p.startTime = time.Now()
	p.filesSeen.Store(0)
	p.filesHashed.Store(0)
	p.bytesProcessed.Store(0)
	p.lastPath.Store("")
	p.state.Store("idle")
}

func (p *Progress) SetState(state string) { p.state.Store(state) }

func (p *Progress) State() string {
	if v := p.state.Load(); v != nil {
		return v.(string)
	}
	return "unknown"
}

func (p *Progress) FileSeen(sizeBytes int64) {
	p.filesSeen.Add(1)
	p.bytesProcessed.Add(sizeBytes)
}

func (p *Progress) FileHashed() { p.filesHashed.Add(1) }

func (p *Progress) SetLastPath(path string) { p.lastPath.Store(path) }

type ProgressSnapshot struct {
	State          string
	FilesSeen      int64
	FilesHashed    int64
	BytesProcessed int64
	ElapsedSeconds int64
	LastPath       string
	MBProcessed    float64
	FilesPerSecond float64
}

func (p *Progress) Snapshot() ProgressSnapshot {
	elapsed := time.Since(p.startTime).Seconds()
	if elapsed < 1 {
		elapsed = 1
	}

	last := ""
	if v := p.lastPath.Load(); v != nil {
		last = v.(string)
	}

	files := p.filesSeen.Load()
	bytes := p.bytesProcessed.Load()

	return ProgressSnapshot{
		State:          p.State(),
		FilesSeen:      files,
		FilesHashed:    p.filesHashed.Load(),
		BytesProcessed: bytes,
		ElapsedSeconds: int64(elapsed),
		LastPath:       last,
		MBProcessed:    float64(bytes) / (1024 * 1024),
		FilesPerSecond: float64(files) / elapsed,
	}
}
