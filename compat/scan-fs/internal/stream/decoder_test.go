package stream

import (
	"context"
	"strings"
	"testing"
)

func TestDecodeLines_timestampsAndExifExtra(t *testing.T) {
	t.Parallel()
	const line = `{"path":"/a/b.jpg","size":9,"modified_at":"2026-04-06T12:00:00Z","exif":{"camera":"Nikon","FocalLength":"50mm"}}` + "\n"
	ctx := context.Background()
	ch := DecodeLines(ctx, strings.NewReader(line))
	res := <-ch
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if res.Record.ModifiedAt == nil || *res.Record.ModifiedAt != "2026-04-06T12:00:00Z" {
		t.Fatalf("modified_at %+v", res.Record.ModifiedAt)
	}
	if res.Record.Exif == nil || res.Record.Exif.Camera == nil || *res.Record.Exif.Camera != "Nikon" {
		t.Fatalf("exif %+v", res.Record.Exif)
	}
	if res.Record.Exif.Extra == nil || string(res.Record.Exif.Extra["FocalLength"]) != `"50mm"` {
		t.Fatalf("exif extra %v", res.Record.Exif.Extra)
	}
}

func TestDecodeLines_minimalRecord(t *testing.T) {
	t.Parallel()
	const line = `{"path":"/tmp/a","size":3}` + "\n"
	ctx := context.Background()
	ch := DecodeLines(ctx, strings.NewReader(line))
	res, ok := <-ch
	if !ok {
		t.Fatal("expected one result")
	}
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if res.Record.Path != "/tmp/a" || res.Record.Size != 3 {
		t.Fatalf("got %+v", res.Record)
	}
	_, ok = <-ch
	if ok {
		t.Fatal("expected channel closed")
	}
}

func TestDecodeLines_largeLineUsesReadBytes(t *testing.T) {
	t.Parallel()
	// Regression: default bufio.Scanner 64KB cap would break; ReadBytes has no fixed cap.
	padding := make([]byte, 70000)
	for i := range padding {
		padding[i] = 'x'
	}
	rec := `{"path":"/big","size":1,"error":"` + string(padding) + `"}` + "\n"
	ctx := context.Background()
	ch := DecodeLines(ctx, strings.NewReader(rec))
	res := <-ch
	if res.Err != nil {
		t.Fatal(res.Err)
	}
	if res.Record.Path != "/big" {
		t.Fatalf("path %q", res.Record.Path)
	}
	if res.Record.Error == nil || len(*res.Record.Error) < 70000 {
		t.Fatalf("expected large error field")
	}
}
