//! Serializable per-file scan result (align with downstream `core/model/file.go` when available).
//!
//! # Filesystem timestamps (`FileRecord`)
//!
//! Optional fields `modified_at`, `accessed_at`, and `created_at` are filled from the same
//! [`std::fs::Metadata`] used for `size`. They are omitted from JSON when unavailable
//! (`#[serde(skip_serializing_if = "Option::is_none")]`). Wire format is RFC3339 in UTC
//! (e.g. `2026-04-06T12:34:56.789Z`).
//!
//! - **`modified_at`** — Last content modification time (POSIX `mtime`).
//! - **`accessed_at`** — Last access time when the platform exposes it; omitted on `Err` from
//!   [`Metadata::accessed`](std::fs::Metadata::accessed). On Linux, `relatime` and similar mount
//!   options may mean atime is not updated on every read.
//! - **`created_at`** — Best-effort creation / birth time from [`Metadata::created`](std::fs::Metadata::created):
//!   Windows file creation time when available; on Unix, birth time when the OS reports it.
//!   Omitted when unsupported. This is **not** inode change time (`ctime`); we do not fake
//!   `created_at` from `ctime`.

use chrono::{DateTime, Utc};
use serde::{Deserialize, Serialize};
use std::collections::HashMap;
use std::time::SystemTime;

/// Converts [`SystemTime`] to UTC [`DateTime`] for JSON serialization.
///
/// Returns [`None`] if the instant is before the Unix epoch or out of range for `chrono`.
pub fn utc_from_system_time(st: SystemTime) -> Option<DateTime<Utc>> {
    let duration = st.duration_since(std::time::UNIX_EPOCH).ok()?;
    DateTime::from_timestamp(duration.as_secs() as i64, duration.subsec_nanos())
}

/// Core document properties from DOCX `docProps/core.xml` (subset).
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct DocxCore {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub creator: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub last_modified_by: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub revision: Option<String>,
}

/// PDF metadata subset from `lopdf`.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct PdfInfo {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub page_count: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub author: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,
}

/// Loose EXIF / image metadata bag for JSON output.
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct ExifInfo {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub camera: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub lens: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub gps_lat: Option<f64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub gps_lon: Option<f64>,
    #[serde(flatten)]
    pub extra: HashMap<String, serde_json::Value>,
}

/// Audio tags summary (from lofty).
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct AudioInfo {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub artist: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub title: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub album: Option<String>,
}

/// Video stream summary from `ffprobe` JSON (minimal).
#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct VideoInfo {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub codec_name: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub width: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub height: Option<u32>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub duration_secs: Option<f64>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct FileRecord {
    pub path: String,
    pub size: u64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub modified_at: Option<DateTime<Utc>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub accessed_at: Option<DateTime<Utc>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub created_at: Option<DateTime<Utc>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub xxhash: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sha256: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub mime: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub format: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub magic_ok: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub phash: Option<u64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub exif: Option<ExifInfo>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub audio: Option<AudioInfo>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub video: Option<VideoInfo>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub pdf: Option<PdfInfo>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub docx: Option<DocxCore>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

#[cfg(test)]
mod tests {
    use super::*;
    use chrono::TimeZone;

    #[test]
    fn serialized_modified_at_is_rfc3339_utc_z() {
        let fixed = Utc.with_ymd_and_hms(2026, 4, 6, 12, 34, 56).unwrap();
        let record = FileRecord {
            path: "/tmp/x".into(),
            size: 0,
            modified_at: Some(fixed),
            ..Default::default()
        };
        let json = serde_json::to_string(&record).unwrap();
        assert!(json.contains("\"modified_at\""));
        assert!(json.contains("2026-04-06T12:34:56"));
        assert!(json.contains('Z'));
        let parsed: FileRecord = serde_json::from_str(&json).unwrap();
        assert_eq!(parsed.modified_at, Some(fixed));
    }
}
