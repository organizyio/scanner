use crate::error::ScanError;
use format::sniff_file;
use hash::{hash_file_sha256_hex, hash_file_xxhash64_hex, partial_digest, PartialOptions};
use metadata::{
    read_audio_tags, read_docx_core, read_exif, read_pdf_info, read_video_ffprobe, AudioMeta,
    DocxMeta, ExifMeta, FfprobeOptions, PdfMeta, VideoMeta,
};
use output::{
    utc_from_system_time, write_record_line, AudioInfo, DocxCore, ExifInfo, FileRecord, PdfInfo,
    VideoInfo,
};
use phash::phash_u64;
use std::fs::Metadata;
use std::io::Write;
use std::path::{Path, PathBuf};
use walker::{walk_roots_fn, FilterOptions};

/// Thin handle around [`ScanOptions`] for callers that prefer an object API.
#[derive(Debug, Clone)]
pub struct Scanner {
    pub options: ScanOptions,
}

impl Scanner {
    pub fn new(options: ScanOptions) -> Self {
        Self { options }
    }

    pub fn scan<W: Write>(&self, writer: &mut W) -> Result<(), ScanError> {
        scan(&self.options, writer)
    }
}

#[derive(Debug, Clone)]
pub enum XxhashMode {
    Off,
    Full,
    /// Head/tail digest for large-file tiering (same hex field as full xxhash on `FileRecord`).
    Partial(PartialOptions),
}

#[derive(Debug, Clone)]
pub struct ScanOptions {
    pub roots: Vec<PathBuf>,
    pub filter: FilterOptions,
    pub xxhash: XxhashMode,
    pub sha256: bool,
    pub sniff_format: bool,
    pub metadata_static: bool,
    pub video_ffprobe: bool,
    pub phash: bool,
}

impl Default for ScanOptions {
    fn default() -> Self {
        Self {
            roots: Vec::new(),
            filter: FilterOptions::default(),
            xxhash: XxhashMode::Off,
            sha256: false,
            sniff_format: false,
            metadata_static: false,
            video_ffprobe: false,
            phash: false,
        }
    }
}

/// Walk `opts.roots` and write one NDJSON line per file to `writer`.
pub fn scan<W: Write>(opts: &ScanOptions, writer: &mut W) -> Result<(), ScanError> {
    let (tx, rx) = std::sync::mpsc::channel();
    walk_roots_fn(&opts.roots, &opts.filter, {
        let tx = tx.clone();
        let opts = opts.clone();
        move |entry| {
            let path = entry.path().to_path_buf();
            let rec = build_record(&path, &opts);
            let _ = tx.send(rec);
        }
    })?;
    drop(tx);
    for rec in rx {
        write_record_line(writer, &rec).map_err(ScanError::Io)?;
    }
    Ok(())
}

fn apply_fs_timestamps(r: &mut FileRecord, meta: &Metadata) {
    if let Ok(t) = meta.modified() {
        r.modified_at = utc_from_system_time(t);
    }
    if let Ok(t) = meta.accessed() {
        r.accessed_at = utc_from_system_time(t);
    }
    if let Ok(t) = meta.created() {
        r.created_at = utc_from_system_time(t);
    }
}

fn build_record(path: &Path, opts: &ScanOptions) -> FileRecord {
    let mut r = FileRecord {
        path: path.display().to_string(),
        size: 0,
        modified_at: None,
        accessed_at: None,
        created_at: None,
        xxhash: None,
        sha256: None,
        mime: None,
        format: None,
        magic_ok: None,
        phash: None,
        exif: None,
        audio: None,
        video: None,
        pdf: None,
        docx: None,
        error: None,
    };

    let meta = match std::fs::metadata(path) {
        Ok(m) => m,
        Err(e) => {
            r.error = Some(e.to_string());
            return r;
        }
    };
    r.size = meta.len();
    apply_fs_timestamps(&mut r, &meta);

    if opts.sniff_format {
        match sniff_file(path) {
            Ok(s) => {
                r.mime = Some(s.media_type);
                r.format = Some(s.format_short_name);
                r.magic_ok = Some(s.extension_matches);
            }
            Err(e) => {
                r.error = Some(format!("format: {e}"));
            }
        }
    }

    match &opts.xxhash {
        XxhashMode::Off => {}
        XxhashMode::Full => match hash_file_xxhash64_hex(path) {
            Ok(h) => r.xxhash = Some(h),
            Err(e) => r.error = Some(format!("xxhash: {e}")),
        },
        XxhashMode::Partial(po) => match partial_digest(path, po) {
            Ok(h) => r.xxhash = Some(h),
            Err(e) => r.error = Some(format!("xxhash partial: {e}")),
        },
    }

    if opts.sha256 {
        match hash_file_sha256_hex(path) {
            Ok(h) => r.sha256 = Some(h),
            Err(e) => r.error = Some(format!("sha256: {e}")),
        }
    }

    if opts.metadata_static {
        if is_image_path(path) {
            if let Some(e) = read_exif(path) {
                r.exif = Some(map_exif(e));
            }
        }
        if is_audio_path(path) {
            if let Some(a) = read_audio_tags(path) {
                r.audio = Some(map_audio(a));
            }
        }
        if path.extension().and_then(|e| e.to_str()) == Some("pdf") {
            if let Some(p) = read_pdf_info(path) {
                r.pdf = Some(map_pdf(p));
            }
        }
        if path.extension().and_then(|e| e.to_str()) == Some("docx") {
            if let Some(d) = read_docx_core(path) {
                r.docx = Some(map_docx(d));
            }
        }
    }

    if opts.video_ffprobe && is_video_path(path) {
        if let Some(v) = read_video_ffprobe(path, &FfprobeOptions::default()) {
            r.video = Some(map_video(v));
        }
    }

    if opts.phash && is_image_path(path) {
        match phash_u64(path) {
            Ok(h) => r.phash = Some(h),
            Err(e) => {
                if r.error.is_none() {
                    r.error = Some(format!("phash: {e}"));
                }
            }
        }
    }

    r
}

fn map_exif(m: ExifMeta) -> ExifInfo {
    ExifInfo {
        camera: m.camera,
        lens: m.lens,
        gps_lat: m.gps_lat,
        gps_lon: m.gps_lon,
        extra: Default::default(),
    }
}

fn map_audio(m: AudioMeta) -> AudioInfo {
    AudioInfo {
        artist: m.artist,
        title: m.title,
        album: m.album,
    }
}

fn map_pdf(m: PdfMeta) -> PdfInfo {
    PdfInfo {
        page_count: m.page_count,
        author: m.author,
        title: m.title,
    }
}

fn map_docx(m: DocxMeta) -> DocxCore {
    DocxCore {
        creator: m.creator,
        last_modified_by: m.last_modified_by,
        revision: m.revision,
    }
}

fn map_video(m: VideoMeta) -> VideoInfo {
    VideoInfo {
        codec_name: m.codec_name,
        width: m.width,
        height: m.height,
        duration_secs: m.duration_secs,
    }
}

fn is_image_path(p: &Path) -> bool {
    matches!(
        p.extension()
            .and_then(|e| e.to_str())
            .map(|e| e.to_ascii_lowercase())
            .as_deref(),
        Some("jpg")
            | Some("jpeg")
            | Some("png")
            | Some("webp")
            | Some("tif")
            | Some("tiff")
            | Some("heic")
    )
}

fn is_audio_path(p: &Path) -> bool {
    matches!(
        p.extension()
            .and_then(|e| e.to_str())
            .map(|e| e.to_ascii_lowercase())
            .as_deref(),
        Some("mp3") | Some("flac") | Some("m4a") | Some("ogg") | Some("opus") | Some("wav")
    )
}

fn is_video_path(p: &Path) -> bool {
    matches!(
        p.extension()
            .and_then(|e| e.to_str())
            .map(|e| e.to_ascii_lowercase())
            .as_deref(),
        Some("mp4") | Some("mkv") | Some("mov") | Some("webm") | Some("avi")
    )
}

#[cfg(test)]
mod tests {
    use super::*;
    use std::fs;
    use tempfile::NamedTempFile;

    #[test]
    fn apply_fs_timestamps_sets_modified_at_for_temp_file() {
        let f = NamedTempFile::new().unwrap();
        let meta = fs::metadata(f.path()).unwrap();
        let mut r = FileRecord::default();
        apply_fs_timestamps(&mut r, &meta);
        assert!(
            r.modified_at.is_some(),
            "mtime should be available for a normal file"
        );
        let line = serde_json::to_string(&r).unwrap();
        assert!(line.contains("modified_at"));
        assert!(line.contains('Z'));
    }
}
