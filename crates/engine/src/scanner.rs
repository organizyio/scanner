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
use std::sync::atomic::{AtomicBool, Ordering};
use std::sync::{mpsc, Arc};
use std::time::Duration;
use walker::{walk_roots_cancellable_fn, FilterOptions, WalkMode};

const RECORD_QUEUE_CAPACITY: usize = 256;
const CANCEL_POLL_INTERVAL: Duration = Duration::from_millis(100);

struct CancelOnDrop(Arc<AtomicBool>);
impl Drop for CancelOnDrop {
    fn drop(&mut self) {
        self.0.store(true, Ordering::Relaxed);
    }
}

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

/// Stream records through a bounded queue, invoking callbacks on the calling thread.
///
/// Cancellation is polled while awaiting records and before delivery. Returned
/// counts include only records successfully accepted by `on_record`. Progress
/// reports the pending record before delivery, so its final count can exceed the
/// returned count when progress requests cancellation. Active extraction and
/// filesystem calls are not preempted; all traversal workers join before return.
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
    consume_records(
        |tx, cancelled| {
            walk_roots_cancellable_fn(
                &opts.roots,
                &opts.filter,
                opts.walk_mode,
                cancelled.clone(),
                {
                    let opts = opts.clone();
                    move |path| {
                        if let Some(rec) = build_record(&path, &opts, &cancelled) {
                            if tx.send(rec).is_err() {
                                cancelled.store(true, Ordering::Relaxed);
                            }
                        }
                    }
                },
            )?;
            Ok(())
        },
        should_stop,
        on_progress,
        on_record,
    )
}

fn consume_records<Producer, ShouldStop, OnProgress, OnRecord>(
    producer: Producer,
    should_stop: &mut ShouldStop,
    on_progress: &mut OnProgress,
    on_record: &mut OnRecord,
) -> Result<(u64, u64), ScanError>
where
    Producer: FnOnce(mpsc::SyncSender<FileRecord>, Arc<AtomicBool>) -> Result<(), ScanError> + Send,
    ShouldStop: FnMut() -> bool,
    OnProgress: FnMut(ScanProgress),
    OnRecord: FnMut(FileRecord) -> Result<(), ScanError>,
{
    if should_stop() {
        return Ok((0, 0));
    }
    std::thread::scope(|scope| {
        // Keep receiver and guard inside the scope closure: both are dropped before
        // scoped threads are joined, including when a caller callback panics.
        let (tx, rx) = mpsc::sync_channel(RECORD_QUEUE_CAPACITY);
        let cancelled = Arc::new(AtomicBool::new(false));
        let stop_guard = CancelOnDrop(cancelled.clone());
        let worker = scope.spawn(move || producer(tx, cancelled));
        let mut files_scanned = 0;
        let mut bytes_scanned = 0;
        let result = loop {
            if should_stop() {
                break Ok((files_scanned, bytes_scanned));
            }
            let rec = match rx.recv_timeout(CANCEL_POLL_INTERVAL) {
                Ok(rec) => rec,
                Err(mpsc::RecvTimeoutError::Timeout) => continue,
                Err(mpsc::RecvTimeoutError::Disconnected) => {
                    break Ok((files_scanned, bytes_scanned))
                }
            };
            if should_stop() {
                break Ok((files_scanned, bytes_scanned));
            }
            let size = rec.identity.size;
            on_progress(ScanProgress {
                files_scanned: files_scanned + 1,
                bytes_scanned: bytes_scanned + size,
                current_path: rec.identity.path.clone(),
            });
            if should_stop() {
                break Ok((files_scanned, bytes_scanned));
            }
            if let Err(error) = on_record(rec) {
                break Err(error);
            }
            files_scanned += 1;
            bytes_scanned += size;
        };
        drop(stop_guard);
        drop(rx);
        let producer_result = worker
            .join()
            .unwrap_or_else(|panic| std::panic::resume_unwind(panic));
        result.and_then(|counts| producer_result.map(|()| counts))
    })
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

fn build_record(path: &Path, opts: &ScanOptions, cancelled: &AtomicBool) -> Option<FileRecord> {
    let check_cancelled = || {
        if cancelled.load(Ordering::Relaxed) {
            None
        } else {
            Some(())
        }
    };
    check_cancelled()?;
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
            return Some(r);
        }
    };
    r.identity.size = meta.len();
    apply_fs_timestamps(&mut r, &meta);

    check_cancelled()?;
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

    check_cancelled()?;
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

    check_cancelled()?;
    if opts.sha256 {
        match hash_file_sha256_hex(path) {
            Ok(h) => {
                let hashes = r.hashes.get_or_insert_with(HashesInfo::default);
                hashes.sha256 = Some(h);
            }
            Err(e) => r.error = Some(format!("sha256: {e}")),
        }
    }
    check_cancelled()?;
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
        check_cancelled()?;
        if is_image_path(path) {
            if let Some(e) = read_exif(path) {
                r.meta.get_or_insert_with(MetaInfo::default).exif = Some(map_exif(e));
            }
        }
        check_cancelled()?;
        if is_audio_path(path) {
            if let Some(a) = read_audio_tags(path) {
                r.meta.get_or_insert_with(MetaInfo::default).audio = Some(map_audio(a));
            }
        }
        check_cancelled()?;
        if path.extension().and_then(|e| e.to_str()) == Some("pdf") {
            if let Some(p) = read_pdf_info(path) {
                r.meta.get_or_insert_with(MetaInfo::default).pdf = Some(map_pdf(p));
            }
        }
        check_cancelled()?;
        if path.extension().and_then(|e| e.to_str()) == Some("docx") {
            if let Some(d) = read_docx_core(path) {
                r.meta.get_or_insert_with(MetaInfo::default).docx = Some(map_docx(d));
            }
        }
    }

    check_cancelled()?;
    if opts.video_ffprobe && is_video_path(path) {
        if let Some(v) = read_video_ffprobe(path, &FfprobeOptions::default()) {
            r.meta.get_or_insert_with(MetaInfo::default).video = Some(map_video(v));
        }
    }

    check_cancelled()?;
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

    check_cancelled()?;
    Some(r)
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

    fn record() -> FileRecord {
        let mut rec = FileRecord::default();
        rec.identity.size = 3;
        rec
    }

    #[test]
    fn delivers_before_producer_finishes_on_calling_thread_without_send_callbacks() {
        use std::cell::Cell;
        use std::rc::Rc;
        let caller = std::thread::current().id();
        let delivered = Rc::new(Cell::new(false));
        let state = delivered.clone();
        let (ack_tx, ack_rx) = mpsc::channel();
        let counts = consume_records(
            move |tx, _| {
                tx.send(record()).unwrap();
                ack_rx.recv_timeout(Duration::from_secs(5)).unwrap();
                Ok(())
            },
            &mut || false,
            &mut |_| assert_eq!(std::thread::current().id(), caller),
            &mut |_| {
                assert_eq!(std::thread::current().id(), caller);
                state.set(true);
                ack_tx.send(()).unwrap();
                Ok(())
            },
        )
        .unwrap();
        assert!(delivered.get());
        assert_eq!(counts, (1, 3));
    }

    #[test]
    fn queue_is_bounded_and_callback_failure_joins_blocked_producer() {
        let attempted = Arc::new(std::sync::atomic::AtomicUsize::new(0));
        let finished = Arc::new(AtomicBool::new(false));
        let attempts = attempted.clone();
        let done = finished.clone();
        let (full_tx, full_rx) = mpsc::channel();
        let result = consume_records(
            move |tx, _| {
                for i in 0..1000 {
                    attempts.store(i + 1, Ordering::SeqCst);
                    if i == RECORD_QUEUE_CAPACITY + 1 {
                        full_tx.send(()).unwrap();
                    }
                    if tx.send(record()).is_err() {
                        break;
                    }
                }
                done.store(true, Ordering::SeqCst);
                Ok(())
            },
            &mut || false,
            &mut |_| {},
            &mut |_| {
                // One record is held by this callback, 256 are queued and the
                // next send is attempted but cannot complete until we return.
                full_rx.recv_timeout(Duration::from_secs(5)).unwrap();
                assert_eq!(attempted.load(Ordering::SeqCst), RECORD_QUEUE_CAPACITY + 2);
                Err(std::io::Error::other("callback failure").into())
            },
        );
        assert!(result.is_err());
        assert!(finished.load(Ordering::SeqCst));
    }

    #[test]
    fn cancellation_while_queue_empty_stops_and_joins_producer() {
        let finished = Arc::new(AtomicBool::new(false));
        let done = finished.clone();
        let start = std::time::Instant::now();
        let mut polls = 0;
        let result = consume_records(
            move |_tx, cancelled| {
                while !cancelled.load(Ordering::Relaxed) {
                    std::thread::yield_now();
                }
                done.store(true, Ordering::SeqCst);
                Ok(())
            },
            &mut || {
                polls += 1;
                polls > 2
            },
            &mut |_| panic!("unexpected progress"),
            &mut |_| panic!("unexpected record"),
        )
        .unwrap();
        assert_eq!(result, (0, 0));
        assert!(finished.load(Ordering::SeqCst));
        assert!(start.elapsed() < Duration::from_secs(5));
    }

    #[test]
    fn cancellation_returns_only_successfully_delivered_counts() {
        let delivered = std::cell::Cell::new(0);
        let result = consume_records(
            |tx, _| {
                for _ in 0..1000 {
                    if tx.send(record()).is_err() {
                        break;
                    }
                }
                Ok(())
            },
            &mut || delivered.get() == 2,
            &mut |_| {},
            &mut |_| {
                delivered.set(delivered.get() + 1);
                Ok(())
            },
        )
        .unwrap();
        assert_eq!(result, (2, 6));
    }

    #[test]
    fn producer_error_is_returned_after_join() {
        let result = consume_records(
            |_tx, _| Err(std::io::Error::other("producer failure").into()),
            &mut || false,
            &mut |_| {},
            &mut |_| Ok(()),
        );
        assert!(result.unwrap_err().to_string().contains("producer failure"));
    }

    #[test]
    fn progress_cancellation_does_not_count_or_deliver_record() {
        let stop = std::cell::Cell::new(false);
        let result = consume_records(
            |tx, _| {
                let _ = tx.send(record());
                Ok(())
            },
            &mut || stop.get(),
            &mut |_| stop.set(true),
            &mut |_| panic!("record delivered after cancellation"),
        )
        .unwrap();
        assert_eq!(result, (0, 0));
    }

    #[test]
    fn callback_panic_unblocks_and_joins_producer() {
        let finished = Arc::new(AtomicBool::new(false));
        let done = finished.clone();
        let result = std::panic::catch_unwind(|| {
            let _ = consume_records(
                move |tx, _| {
                    while tx.send(record()).is_ok() {}
                    done.store(true, Ordering::SeqCst);
                    Ok(())
                },
                &mut || false,
                &mut |_| {},
                &mut |_| panic!("callback panic"),
            );
        });
        assert!(result.is_err());
        assert!(finished.load(Ordering::SeqCst));
    }

    #[test]
    fn pre_cancelled_scan_does_not_start_producer() {
        let result = consume_records(
            |_tx, _| -> Result<(), ScanError> { panic!("producer started after cancellation") },
            &mut || true,
            &mut |_| panic!("unexpected progress"),
            &mut |_| panic!("unexpected record"),
        )
        .unwrap();
        assert_eq!(result, (0, 0));
    }

    #[test]
    fn cancelled_extraction_produces_no_record() {
        assert!(build_record(
            Path::new("missing"),
            &ScanOptions::default(),
            &AtomicBool::new(true)
        )
        .is_none());
    }

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
