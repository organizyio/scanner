package stream

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"io"

	"github.com/meshory/scan-fs/pkg/item"
)

// LineResult is one decoded NDJSON line or a terminal error.
type LineResult struct {
	Record item.FileRecord
	Err    error
}

// DecodeLines reads NDJSON from r and sends results on the returned channel until EOF or error.
func DecodeLines(ctx context.Context, r io.Reader) <-chan LineResult {
	ch := make(chan LineResult, 32)
	go func() {
		defer close(ch)

		br := bufio.NewReader(r)
		for {
			if ctx.Err() != nil {
				ch <- LineResult{Err: ctx.Err()}
				return
			}
			line, err := br.ReadBytes('\n')
			if len(line) > 0 && line[len(line)-1] == '\n' {
				line = line[:len(line)-1]
			}
			if len(line) == 0 && err == io.EOF {
				return
			}
			if err != nil && err != io.EOF {
				ch <- LineResult{Err: err}
				return
			}
			if len(line) == 0 {
				if err == io.EOF {
					return
				}
				continue
			}
			var rec item.FileRecord
			if uerr := json.Unmarshal(line, &rec); uerr != nil {
				ch <- LineResult{Err: fmt.Errorf("decode ndjson: %w", uerr)}
				return
			}
			select {
			case <-ctx.Done():
				ch <- LineResult{Err: ctx.Err()}
				return
			case ch <- LineResult{Record: rec}:
			}
			if err == io.EOF {
				return
			}
		}
	}()
	return ch
}
