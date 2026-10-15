use engine::{scan, ScanOptions, XxhashMode};
use std::path::PathBuf;
use tempfile::tempdir;
use walker::FilterOptions;

#[test]
fn scan_emits_ndjson_line() {
    let dir = tempdir().unwrap();
    let f = dir.path().join("hello.txt");
    std::fs::write(&f, b"hello scan-engine").unwrap();

    let opts = ScanOptions {
        roots: vec![dir.path().to_path_buf()],
        filter: FilterOptions::default(),
        xxhash: XxhashMode::Full,
        sha256: false,
        sniff_format: false,
        metadata_static: false,
        video_ffprobe: false,
        phash: false,
    };

    let mut buf = Vec::new();
    scan(&opts, &mut buf).unwrap();
    let line = String::from_utf8(buf).unwrap();
    let v: serde_json::Value = serde_json::from_str(line.trim()).unwrap();
    assert_eq!(v["path"], serde_json::json!(path_to_json_string(&f)));
    assert!(v["xxhash"].as_str().is_some());
    assert_eq!(v["size"], serde_json::json!(17));
}

fn path_to_json_string(p: &std::path::Path) -> String {
    PathBuf::from(p).display().to_string()
}
