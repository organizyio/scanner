// Package item defines scan result types aligned with scan-engine output::FileRecord.
package item

import "encoding/json"

// DocxCore mirrors scan-engine output::DocxCore.
type DocxCore struct {
	Creator        *string `json:"creator,omitempty"`
	LastModifiedBy *string `json:"last_modified_by,omitempty"`
	Revision       *string `json:"revision,omitempty"`
}

// PdfInfo mirrors scan-engine output::PdfInfo.
type PdfInfo struct {
	PageCount *uint32 `json:"page_count,omitempty"`
	Author    *string `json:"author,omitempty"`
	Title     *string `json:"title,omitempty"`
}

// ExifInfo mirrors scan-engine output::ExifInfo.
// The engine stores ad hoc EXIF keys in a serde-flattened map; see UnmarshalJSON/MarshalJSON in exif_json.go.
type ExifInfo struct {
	Camera *string  `json:"camera,omitempty"`
	Lens   *string  `json:"lens,omitempty"`
	GpsLat *float64 `json:"gps_lat,omitempty"`
	GpsLon *float64 `json:"gps_lon,omitempty"`
	// Extra holds every other key from the exif object (serde #[serde(flatten)] extra in Rust).
	Extra map[string]json.RawMessage `json:"-"`
}

// AudioInfo mirrors scan-engine output::AudioInfo.
type AudioInfo struct {
	Artist *string `json:"artist,omitempty"`
	Title  *string `json:"title,omitempty"`
	Album  *string `json:"album,omitempty"`
}

// VideoInfo mirrors scan-engine output::VideoInfo.
type VideoInfo struct {
	CodecName    *string  `json:"codec_name,omitempty"`
	Width        *uint32  `json:"width,omitempty"`
	Height       *uint32  `json:"height,omitempty"`
	DurationSecs *float64 `json:"duration_secs,omitempty"`
}

// FileRecord is one NDJSON line from scan-engine.
type FileRecord struct {
	Path       string     `json:"path"`
	Size       uint64     `json:"size"`
	ModifiedAt *string    `json:"modified_at,omitempty"`
	AccessedAt *string    `json:"accessed_at,omitempty"`
	CreatedAt  *string    `json:"created_at,omitempty"`
	Xxhash     *string    `json:"xxhash,omitempty"`
	Sha256     *string    `json:"sha256,omitempty"`
	Mime       *string    `json:"mime,omitempty"`
	Format     *string    `json:"format,omitempty"`
	MagicOk    *bool      `json:"magic_ok,omitempty"`
	Phash      *uint64    `json:"phash,omitempty"`
	Exif       *ExifInfo  `json:"exif,omitempty"`
	Audio      *AudioInfo `json:"audio,omitempty"`
	Video      *VideoInfo `json:"video,omitempty"`
	Pdf        *PdfInfo   `json:"pdf,omitempty"`
	Docx       *DocxCore  `json:"docx,omitempty"`
	Error      *string    `json:"error,omitempty"`
}

// Item is a normalized file record for scan-fs output.
type Item struct {
	FileRecord
	Source    string `json:"source"`
	Device    string `json:"device"`
	Directory string `json:"directory"`
	Name      string `json:"name"`
	Extension string `json:"extension"`
}
