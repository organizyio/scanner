use clap::{Parser, ValueEnum};
use std::path::PathBuf;

#[derive(Parser, Debug)]
#[command(name = "scanfs")]
#[command(about = "Filesystem scanner: paths, JSON lines, or table output (Archivist scanfs).")]
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

    /// Compute MD5 (hex) for compatibility with external dedup tools.
    #[arg(long, default_value_t = false)]
    pub md5: bool,

    /// Extraction profile controlling default depth.
    #[arg(long, value_enum, default_value_t = ScanProfile::Standard)]
    pub profile: ScanProfile,

    /// Sniff magic bytes / MIME.
    #[arg(long = "sniff-format", default_value_t = false)]
    pub sniff_format: bool,

    /// Read EXIF / audio tags / PDF page count / DOCX core props.
    #[arg(long, default_value_t = false)]
    pub metadata: bool,

    /// Run `ffprobe` on video extensions (requires binary on PATH; 30s timeout).
    #[arg(long, default_value_t = false)]
    pub video: bool,

    /// Perceptual hash (images only).
    #[arg(long, default_value_t = false)]
    pub phash: bool,

    /// Output format for normalized Item records.
    #[arg(long, value_enum, default_value_t = OutputFormat::Plain)]
    pub format: OutputFormat,

    /// Worker threads for parallel traversal. If > 0, sets `RAYON_NUM_THREADS`.
    #[arg(long, default_value_t = 0)]
    pub workers: usize,
}

#[derive(Copy, Clone, Debug, Eq, PartialEq, ValueEnum)]
pub enum OutputFormat {
    Plain,
    Json,
    Table,
}

#[derive(Copy, Clone, Debug, Eq, PartialEq, ValueEnum)]
pub enum ScanProfile {
    Fast,
    Standard,
    Deep,
}
