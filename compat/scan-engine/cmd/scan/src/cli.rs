use clap::Parser;
use engine::{ScanOptions, XxhashMode};
use hash::PartialOptions;
use std::path::PathBuf;
use walker::FilterOptions;

#[derive(Parser, Debug)]
#[command(name = "scan")]
#[command(about = "Walk directories and emit one JSON object per file (NDJSON).")]
pub struct Cli {
    /// Root directories to scan (at least one required).
    #[arg(long = "root", value_name = "PATH", action = clap::ArgAction::Append, required = true)]
    pub roots: Vec<PathBuf>,

    /// Extra ignore-style glob (repeatable); passed to `ignore` overrides.
    #[arg(long = "exclude", value_name = "GLOB", action = clap::ArgAction::Append)]
    pub exclude: Vec<String>,

    /// Compute full-file xxHash64 (hex).
    #[arg(long, default_value_t = false)]
    pub hash: bool,

    /// Use head/tail xxHash64 instead of full file (tiered large files).
    #[arg(long, default_value_t = false)]
    pub hash_partial: bool,

    /// Compute SHA-256 (hex).
    #[arg(long, default_value_t = false)]
    pub sha256: bool,

    /// Sniff magic bytes / MIME (`file-format`).
    #[arg(long, default_value_t = false)]
    pub format: bool,

    /// Read EXIF / audio tags / PDF page count / DOCX core props.
    #[arg(long, default_value_t = false)]
    pub metadata: bool,

    /// Run `ffprobe` on video extensions (requires binary on PATH; 30s timeout).
    #[arg(long, default_value_t = false)]
    pub video: bool,

    /// Perceptual hash (images only).
    #[arg(long, default_value_t = false)]
    pub phash: bool,
}

impl Cli {
    pub fn to_scan_options(&self) -> ScanOptions {
        let filter = FilterOptions {
            overrides: self.exclude.clone(),
            ..Default::default()
        };

        let xxhash = if self.hash_partial {
            XxhashMode::Partial(PartialOptions::default())
        } else if self.hash {
            XxhashMode::Full
        } else {
            XxhashMode::Off
        };

        ScanOptions {
            roots: self.roots.clone(),
            filter,
            xxhash,
            sha256: self.sha256,
            sniff_format: self.format,
            metadata_static: self.metadata,
            video_ffprobe: self.video,
            phash: self.phash,
        }
    }
}
