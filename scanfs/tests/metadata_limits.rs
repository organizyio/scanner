use assert_cmd::cargo::cargo_bin;
use serde_json::Value;
use std::{fs, path::Path, process::Command};
use tempfile::tempdir;
const MANIFEST: &str = include_str!("../../crates/metadata/tests/fixtures/manifest.json");
#[test]
fn budget_declines_preserve_hashes_rows_and_following_successful_metadata() {
    let temp = tempdir().unwrap();
    let root = temp.path().join("文書");
    fs::create_dir(&root).unwrap();
    let fixtures = Path::new(env!("CARGO_MANIFEST_DIR")).join("../crates/metadata/tests/fixtures");
    let spec: Value = serde_json::from_str(MANIFEST).unwrap();
    for name in spec.as_object().unwrap().keys() {
        fs::copy(fixtures.join(name), root.join(name)).unwrap();
    }
    for _ in 0..2 {
        let mut bytes = Vec::new();
        scanner::scan(
            &scanner::ScanOptions {
                roots: vec![root.clone()],
                metadata_static: true,
                sha256: true,
                ..Default::default()
            },
            &mut bytes,
        )
        .unwrap();
        check(&bytes, &spec);
        let out = Command::new(cargo_bin("scanfs"))
            .args([
                "--root",
                root.to_str().unwrap(),
                "--metadata",
                "--sha256",
                "--format",
                "json",
            ])
            .output()
            .unwrap();
        assert!(out.status.success());
        check(&out.stdout, &spec);
    }
}
fn check(bytes: &[u8], spec: &Value) {
    let rows: Vec<Value> = String::from_utf8_lossy(bytes)
        .lines()
        .map(|s| serde_json::from_str(s).unwrap())
        .collect();
    assert_eq!(rows.len(), 6);
    for r in rows {
        let name = Path::new(r["identity"]["path"].as_str().unwrap())
            .file_name()
            .unwrap()
            .to_str()
            .unwrap();
        let expected = &spec[name];
        assert_eq!(r["schema_version"], 1);
        assert!(r["error"].is_null());
        assert_eq!(r["identity"]["size"], expected["bytes"]);
        assert_eq!(r["hashes"]["sha256"], expected["sha256"]);
        match name {
            "valid.docx" => {
                assert_eq!(r["meta"]["docx"]["creator"], "Synthetic & é🙂");
                assert_eq!(r["meta"]["docx"]["revision"], "007");
            }
            "valid.pdf" => assert_eq!(r["meta"]["pdf"]["page_count"], 1),
            "valid.tiff" => {
                let m = &r["meta"]["exif"];
                assert_eq!(m["camera"], "\"Synthetic\"");
                assert!(m
                    .as_object()
                    .unwrap()
                    .values()
                    .any(|v| *v == serde_json::json!([4294967295u64])));
            }
            _ => assert!(r["meta"].is_null(), "{name}: {r}"),
        }
    }
}
