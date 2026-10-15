//! Per-file metadata: EXIF, audio tags, PDF, DOCX, video (`ffprobe`).

mod audio;
mod document;
mod exif;
mod types;
mod video;

pub use audio::read_audio_tags;
pub use document::{read_docx_core, read_pdf_info};
pub use exif::read_exif;
pub use types::{AudioMeta, DocxMeta, ExifMeta, PdfMeta, VideoMeta};
pub use video::{read_video_ffprobe, FfprobeOptions};
