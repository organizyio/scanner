use crate::error::ScanError;
use format::sniff_file;
use hash::{
    hash_file_md5_hex, hash_file_sha256_hex, hash_file_xxhash64_hex, partial_digest, PartialOptions,
};
use metadata::{
    read_audio_tags, read_docx_core, read_exif, read_pdf_info, read_video_ffprobe, AudioMeta,
    DocxMeta, ExifMeta, FfprobeOptions, PdfMeta, VideoMeta,
};
use output::{
    utc_from_system_time, write_record_line, AudioInfo, DocxCore, ExifInfo, FileRecord, FormatInfo,
    HashesInfo, IdentityInfo, MetaInfo, PdfInfo, VideoInfo,
};
use phash::phash_u64;
use std::fs::Metadata;
use std::io::Write;
use std::path::{Path, PathBuf};
use walker::{walk_roots_fn, FilterOptions, WalkMode};

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
    pub walk_mode: WalkMode,
    pub xxhash: XxhashMode,
    pub md5: bool,
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
            walk_mode: WalkMode::Standard,
            xxhash: XxhashMode::Off,
            md5: false,
            sha256: false,
            sniff_format: false,
            metadata_static: false,
            video_ffprobe: false,
            phash: false,
        }
    }
}

#[derive(Debug, Clone, Default)]
pub struct ScanProgress {
    pub files_scanned: u64,
    pub bytes_scanned: u64,
    pub current_path: String,
}

/// Walk `opts.roots` and write one NDJSON line per file to `writer`.
pub fn scan<W: Write>(opts: &ScanOptions, writer: &mut W) -> Result<(), ScanError> {
    let mut should_stop = || false;
    let mut on_progress = |_p: ScanProgress| {};
    let mut on_record = |rec: FileRecord| -> Result<(), ScanError> {
        write_record_line(writer, &rec).map_err(ScanError::Io)
    };
    let _ = scan_with_callbacks(opts, &mut should_stop, &mut on_progress, &mut on_record)?;
    Ok(())
}

pub fn scan_with_callbacks<ShouldStop, OnProgress, OnRecord>(
    opts: &ScanOptions,
    should_stop: &mut ShouldStop,
    on_progress: &mut OnProgress,
    on_record: &mut OnRecord,
) -> Result<(u64, u64), ScanError>
where
    ShouldStop: FnMut() -> bool,
    OnProgress: FnMut(ScanProgress),
    OnRecord: FnMut(FileRecord) -> Result<(), ScanError>,
{
    let (tx, rx) = std::sync::mpsc::channel();
    walk_roots_fn(&opts.roots, &opts.filter, opts.walk_mode, {
        let tx = tx.clone();
        let opts = opts.clone();
        move |path| {
            let rec = build_record(&path, &opts);
            let _ = tx.send(rec);
        }
    })?;
    drop(tx);
    let mut files_scanned: u64 = 0;
    let mut bytes_scanned: u64 = 0;
    for rec in rx {
        if should_stop() {
            break;
        }
        files_scanned += 1;
        bytes_scanned += rec.identity.size;
        on_progress(ScanProgress {
            files_scanned,
            bytes_scanned,
            current_path: rec.identity.path.clone(),
        });
        on_record(rec)?;
    }
    Ok((files_scanned, bytes_scanned))
}

fn apply_fs_timestamps(r: &mut FileRecord, meta: &Metadata) {
    if let Ok(t) = meta.modified() {
        r.identity.modified_at = utc_from_system_time(t);
    }
    if let Ok(t) = meta.accessed() {
        r.identity.accessed_at = utc_from_system_time(t);
    }
    if let Ok(t) = meta.created() {
        r.identity.created_at = utc_from_system_time(t);
    }
    apply_file_identity(r, meta);
}

fn build_record(path: &Path, opts: &ScanOptions) -> FileRecord {
    let mut r = FileRecord {
        schema_version: 1,
        identity: IdentityInfo {
            path: path.display().to_string(),
            size: 0,
            ..Default::default()
        },
        hashes: Some(HashesInfo::default()),
        format: Some(FormatInfo::default()),
        meta: Some(MetaInfo::default()),
        error: None,
    };

    let meta = match std::fs::metadata(path) {
        Ok(m) => m,
        Err(e) => {
            r.error = Some(e.to_string());
            return r;
        }
    };
    r.identity.size = meta.len();
    apply_fs_timestamps(&mut r, &meta);

    if opts.sniff_format {
        match sniff_file(path) {
            Ok(s) => {
                let fmt = r.format.get_or_insert_with(FormatInfo::default);
                fmt.mime = Some(s.media_type);
                fmt.kind = Some(s.format_short_name);
                fmt.extension_match = Some(s.extension_matches);
            }
            Err(e) => {
                r.error = Some(format!("format: {e}"));
            }
        }
    }

    match &opts.xxhash {
        XxhashMode::Off => {
            if let Some(h) = r.hashes.as_mut() {
                h.mode = Some("off".to_string());
            }
        }
        XxhashMode::Full => match hash_file_xxhash64_hex(path) {
            Ok(h) => {
                let hashes = r.hashes.get_or_insert_with(HashesInfo::default);
                hashes.xxhash64 = Some(h);
                hashes.mode = Some("full".to_string());
            }
            Err(e) => r.error = Some(format!("xxhash: {e}")),
        },
        XxhashMode::Partial(po) => match partial_digest(path, po) {
            Ok(h) => {
                let hashes = r.hashes.get_or_insert_with(HashesInfo::default);
                hashes.xxhash64 = Some(h);
                hashes.mode = Some("partial".to_string());
            }
            Err(e) => r.error = Some(format!("xxhash partial: {e}")),
        },
    }

    if opts.sha256 {
        match hash_file_sha256_hex(path) {
            Ok(h) => {
                let hashes = r.hashes.get_or_insert_with(HashesInfo::default);
                hashes.sha256 = Some(h);
            }
            Err(e) => r.error = Some(format!("sha256: {e}")),
        }
    }
    if opts.md5 {
        match hash_file_md5_hex(path) {
            Ok(h) => {
                let hashes = r.hashes.get_or_insert_with(HashesInfo::default);
                hashes.md5 = Some(h);
            }
            Err(e) => r.error = Some(format!("md5: {e}")),
        }
    }

    if opts.metadata_static {
        if is_image_path(path) {
            if let Some(e) = read_exif(path) {
                r.meta.get_or_insert_with(MetaInfo::default).exif = Some(map_exif(e));
            }
        }
        if is_audio_path(path) {
            if let Some(a) = read_audio_tags(path) {
                r.meta.get_or_insert_with(MetaInfo::default).audio = Some(map_audio(a));
            }
        }
        if path.extension().and_then(|e| e.to_str()) == Some("pdf") {
            if let Some(p) = read_pdf_info(path) {
                r.meta.get_or_insert_with(MetaInfo::default).pdf = Some(map_pdf(p));
            }
        }
        if path.extension().and_then(|e| e.to_str()) == Some("docx") {
            if let Some(d) = read_docx_core(path) {
                r.meta.get_or_insert_with(MetaInfo::default).docx = Some(map_docx(d));
            }
        }
    }

    if opts.video_ffprobe && is_video_path(path) {
        if let Some(v) = read_video_ffprobe(path, &FfprobeOptions::default()) {
            r.meta.get_or_insert_with(MetaInfo::default).video = Some(map_video(v));
        }
    }

    if opts.phash && is_image_path(path) {
        match phash_u64(path) {
            Ok(h) => r.meta.get_or_insert_with(MetaInfo::default).phash = Some(h),
            Err(e) => {
                if r.error.is_none() {
                    r.error = Some(format!("phash: {e}"));
                }
            }
        }
    }

    if matches!(
        r.hashes.as_ref(),
        Some(h) if h.xxhash64.is_none() && h.md5.is_none() && h.sha256.is_none() && h.mode.is_none()
    ) {
        r.hashes = None;
    }
    if matches!(
        r.format.as_ref(),
        Some(f) if f.kind.is_none() && f.mime.is_none() && f.extension_match.is_none() && f.confidence.is_none()
    ) {
        r.format = None;
    }
    if matches!(
        r.meta.as_ref(),
        Some(m)
            if m.phash.is_none()
                && m.exif.is_none()
                && m.audio.is_none()
                && m.video.is_none()
                && m.pdf.is_none()
                && m.docx.is_none()
    ) {
        r.meta = None;
    }

    r
}

#[cfg(unix)]
fn apply_file_identity(r: &mut FileRecord, meta: &Metadata) {
    #[cfg(unix)]
    {
        use std::os::unix::fs::MetadataExt;
        r.identity.inode = Some(meta.ino());
        r.identity.device_id = Some(meta.dev());
    }
}

#[cfg(not(unix))]
fn apply_file_identity(_r: &mut FileRecord, _meta: &Metadata) {}

fn map_exif(m: ExifMeta) -> ExifInfo {
    ExifInfo {
        camera: m.camera,
        lens: m.lens,
        gps_lat: m.gps_lat,
        gps_lon: m.gps_lon,
        extra: m.extra,
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
    use std::path::PathBuf;
    use tempfile::tempdir;
    use tempfile::NamedTempFile;

    #[test]
    fn apply_fs_timestamps_sets_modified_at_for_temp_file() {
        let f = NamedTempFile::new().unwrap();
        let meta = fs::metadata(f.path()).unwrap();
        let mut r = FileRecord {
            schema_version: 1,
            ..Default::default()
        };
        r.identity.path = "x".to_string();
        r.identity.size = 0;
        apply_fs_timestamps(&mut r, &meta);
        assert!(
            r.identity.modified_at.is_some(),
            "mtime should be available for a normal file"
        );
        let line = serde_json::to_string(&r).unwrap();
        assert!(line.contains("modified_at"));
        assert!(line.contains('Z'));
    }

    #[test]
    fn standard_and_full_modes_produce_same_record_for_same_file() {
        let dir = tempdir().expect("temp dir");
        let p = dir.path().join("a.txt");
        fs::write(&p, "hello").expect("write file");

        let collect = |mode: WalkMode| -> FileRecord {
            let opts = ScanOptions {
                roots: vec![PathBuf::from(dir.path())],
                walk_mode: mode,
                ..Default::default()
            };
            let mut out: Vec<FileRecord> = Vec::new();
            let mut should_stop = || false;
            let mut on_progress = |_p: ScanProgress| {};
            let mut on_record = |r: FileRecord| -> Result<(), ScanError> {
                out.push(r);
                Ok(())
            };
            let _ = scan_with_callbacks(&opts, &mut should_stop, &mut on_progress, &mut on_record)
                .expect("scan callbacks");
            out.into_iter()
                .find(|r| r.identity.path == p.display().to_string())
                .expect("record for target file")
        };

        let standard = collect(WalkMode::Standard);
        let full = collect(WalkMode::Full);
        let standard_json = serde_json::to_string(&standard).expect("std json");
        let full_json = serde_json::to_string(&full).expect("full json");
        assert_eq!(standard_json, full_json);
    }
}
