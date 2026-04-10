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
pub struct IdentityInfo {
    pub path: String,
    pub size: u64,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub modified_at: Option<DateTime<Utc>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub accessed_at: Option<DateTime<Utc>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub created_at: Option<DateTime<Utc>>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub inode: Option<u64>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub device_id: Option<u64>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct HashesInfo {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub xxhash64: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub md5: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub sha256: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub mode: Option<String>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct FormatInfo {
    #[serde(skip_serializing_if = "Option::is_none")]
    pub kind: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub mime: Option<String>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub extension_match: Option<bool>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub confidence: Option<f64>,
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct MetaInfo {
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
}

#[derive(Debug, Clone, Default, Serialize, Deserialize)]
pub struct FileRecord {
    pub schema_version: u32,
    pub identity: IdentityInfo,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub hashes: Option<HashesInfo>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub format: Option<FormatInfo>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub meta: Option<MetaInfo>,
    #[serde(skip_serializing_if = "Option::is_none")]
    pub error: Option<String>,
}

#[cfg(test)]
mod tests {
    use super::*;
    use chrono::TimeZone;
    use std::collections::HashMap;

    #[test]
    fn serialized_modified_at_is_rfc3339_utc_z() {
        let fixed = Utc.with_ymd_and_hms(2026, 4, 6, 12, 34, 56).unwrap();
        let record = FileRecord {
            schema_version: 1,
            identity: IdentityInfo {
                path: "/tmp/x".into(),
                size: 0,
                modified_at: Some(fixed),
                ..Default::default()
            },
            ..Default::default()
        };
        let json = serde_json::to_string(&record).unwrap();
        assert!(json.contains("\"schema_version\":1"));
        assert!(json.contains("\"identity\""));
        assert!(json.contains("\"modified_at\""));
        assert!(json.contains("2026-04-06T12:34:56"));
        assert!(json.contains('Z'));
        let parsed: FileRecord = serde_json::from_str(&json).unwrap();
        assert_eq!(parsed.identity.modified_at, Some(fixed));
    }

    #[test]
    fn exif_unknown_key_roundtrip_survives() {
        let mut extra = HashMap::new();
        extra.insert(
            "ImageDescription".to_string(),
            serde_json::json!("sample description"),
        );
        let record = FileRecord {
            schema_version: 1,
            identity: IdentityInfo {
                path: "/tmp/x.jpg".into(),
                size: 1,
                ..Default::default()
            },
            meta: Some(MetaInfo {
                exif: Some(ExifInfo {
                    camera: Some("A".into()),
                    lens: None,
                    gps_lat: None,
                    gps_lon: None,
                    extra,
                }),
                ..Default::default()
            }),
            ..Default::default()
        };
        let json = serde_json::to_string(&record).unwrap();
        let parsed: FileRecord = serde_json::from_str(&json).unwrap();
        let exif = parsed.meta.unwrap().exif.unwrap();
        assert_eq!(
            exif.extra.get("ImageDescription"),
            Some(&serde_json::json!("sample description"))
        );
    }

    #[test]
    fn json_deserializes_minimal_ndjson_line() {
        let json = r#"{"schema_version":1,"identity":{"path":"a","size":2}}"#;
        let rec: FileRecord = serde_json::from_str(json).unwrap();
        assert_eq!(rec.identity.path, "a");
        assert_eq!(rec.identity.size, 2);
    }

    #[test]
    fn json_deserializes_large_error_field() {
        let big = "a".repeat(75 * 1024);
        let json = format!(
            r#"{{"schema_version":1,"identity":{{"path":"/tmp/x","size":1}},"error":{}}}"#,
            serde_json::to_string(&big).unwrap()
        );
        let rec: FileRecord = serde_json::from_str(&json).unwrap();
        assert_eq!(rec.identity.path, "/tmp/x");
        assert_eq!(rec.identity.size, 1);
        assert_eq!(rec.error.as_deref(), Some(big.as_str()));
    }
}
