//! Plain structs returned by metadata readers (mapped into `output::FileRecord` by `engine`).
use std::collections::HashMap;

#[derive(Debug, Clone, Default)]
pub struct ExifMeta {
    pub camera: Option<String>,
    pub lens: Option<String>,
    pub gps_lat: Option<f64>,
    pub gps_lon: Option<f64>,
    pub extra: HashMap<String, serde_json::Value>,
}

#[derive(Debug, Clone, Default)]
pub struct AudioMeta {
    pub artist: Option<String>,
    pub title: Option<String>,
    pub album: Option<String>,
}

#[derive(Debug, Clone, Default)]
pub struct PdfMeta {
    pub page_count: Option<u32>,
    pub author: Option<String>,
    pub title: Option<String>,
}

#[derive(Debug, Clone, Default)]
pub struct DocxMeta {
    pub creator: Option<String>,
    pub last_modified_by: Option<String>,
    pub revision: Option<String>,
}

#[derive(Debug, Clone, Default)]
pub struct VideoMeta {
    pub codec_name: Option<String>,
    pub width: Option<u32>,
    pub height: Option<u32>,
    pub duration_secs: Option<f64>,
}
