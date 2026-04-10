use crate::cli::{Cli, ScanProfile};
use scanner::{FilterOptions, PartialOptions, ScanOptions, WalkMode, XxhashMode};

pub fn scan_options_from_cli(cli: &Cli) -> ScanOptions {
    let mut md5 = cli.md5;
    let mut sha256 = cli.sha256;
    if cli.profile == ScanProfile::Deep {
        md5 = true;
        sha256 = true;
    }
    let xxhash = if cli.hash_partial {
        XxhashMode::Partial(PartialOptions::default())
    } else if cli.hash {
        XxhashMode::Full
    } else {
        XxhashMode::Off
    };
    ScanOptions {
        roots: cli.roots.clone(),
        filter: FilterOptions {
            overrides: cli.exclude.clone(),
            ..Default::default()
        },
        walk_mode: WalkMode::Standard,
        xxhash,
        md5,
        sha256,
        sniff_format: cli.sniff_format,
        metadata_static: cli.metadata,
        video_ffprobe: cli.video,
        phash: cli.phash,
    }
}
