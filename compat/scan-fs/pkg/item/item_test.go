package item

import (
	"encoding/json"
	"testing"
)

func TestExifInfo_roundtrip_flattenedExtra(t *testing.T) {
	t.Parallel()
	const in = `{"camera":"Canon","gps_lat":40.71,"ImageDescription":"sunset"}`
	var ex ExifInfo
	if err := json.Unmarshal([]byte(in), &ex); err != nil {
		t.Fatal(err)
	}
	if ex.Camera == nil || *ex.Camera != "Canon" {
		t.Fatalf("camera %+v", ex.Camera)
	}
	if ex.GpsLat == nil || *ex.GpsLat != 40.71 {
		t.Fatalf("gps_lat %+v", ex.GpsLat)
	}
	if ex.Extra == nil || string(ex.Extra["ImageDescription"]) != `"sunset"` {
		t.Fatalf("extra %v", ex.Extra)
	}
	out, err := json.Marshal(&ex)
	if err != nil {
		t.Fatal(err)
	}
	var again ExifInfo
	if err := json.Unmarshal(out, &again); err != nil {
		t.Fatal(err)
	}
	if again.Camera == nil || *again.Camera != "Canon" {
		t.Fatalf("again camera %+v", again.Camera)
	}
	if again.Extra == nil || string(again.Extra["ImageDescription"]) != `"sunset"` {
		t.Fatalf("again extra %v", again.Extra)
	}
}

func TestFileRecord_timestampsRoundTripThroughItem(t *testing.T) {
	t.Parallel()
	mod := "2026-04-06T12:34:56.789Z"
	acc := "2025-01-02T00:00:00Z"
	rec := FileRecord{
		Path:       "/x",
		Size:       1,
		ModifiedAt: &mod,
		AccessedAt: &acc,
	}
	it := Item{
		FileRecord: rec,
		Source:     "src",
		Device:     "/root",
		Directory:  "/",
		Name:       "x",
		Extension:  "",
	}
	data, err := json.Marshal(it)
	if err != nil {
		t.Fatal(err)
	}
	var got Item
	if err := json.Unmarshal(data, &got); err != nil {
		t.Fatal(err)
	}
	if got.ModifiedAt == nil || *got.ModifiedAt != mod {
		t.Fatalf("modified_at %v", got.ModifiedAt)
	}
	if got.AccessedAt == nil || *got.AccessedAt != acc {
		t.Fatalf("accessed_at %v", got.AccessedAt)
	}
	if got.CreatedAt != nil {
		t.Fatalf("created_at %v", got.CreatedAt)
	}
}
