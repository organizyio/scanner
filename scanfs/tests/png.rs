//! Public facade and CLI qualification; no dev dependency may activate codecs.
use assert_cmd::cargo::cargo_bin;
use serde_json::Value;
use std::{fs, path::Path, process::Command};
use tempfile::tempdir;

const GOLDEN: u64 = 0xff00ff00ff00ff00;

#[test]
fn png_encodings_have_exact_gradient_hash_through_facade_and_cli() {
    let dir = tempdir().unwrap();
    let fixtures = Path::new(env!("CARGO_MANIFEST_DIR")).join("../crates/phash/tests/fixtures");
    for name in [
        "rgb.png",
        "rgba.png",
        "gray.png",
        "gray-alpha.png",
        "indexed.png",
        "gray16.png",
        "rgb16.png",
        "gray-alpha16.png",
        "rgba16.png",
        "adam7.png",
        "constant.png",
    ] {
        fs::copy(fixtures.join(name), dir.path().join(name)).unwrap();
    }
    // Uppercase extension, Unicode path, corrupt/truncated PNG, and a valid
    // unsupported JPEG. Failure rows must remain truthful and must not gain a hash.
    fs::copy(fixtures.join("rgb.png"), dir.path().join("画像.PNG")).unwrap();
    let mut corrupt = fs::read(fixtures.join("rgb.png")).unwrap();
    corrupt[50] ^= 0x80;
    fs::write(dir.path().join("corrupt.png"), corrupt).unwrap();
    fs::write(dir.path().join("truncated.png"), b"\x89PNG\r\n\x1a\n").unwrap();
    fs::write(
        dir.path().join("unsupported.jpg"),
        include_bytes!("fixtures/unsupported.jpg"),
    )
    .unwrap();
    let mut bytes = Vec::new();
    scanner::scan(
        &scanner::ScanOptions {
            roots: vec![dir.path().to_owned()],
            phash: true,
            sha256: true,
            ..Default::default()
        },
        &mut bytes,
    )
    .unwrap();
    check(&bytes, true);
    for enabled in [true, false] {
        let mut command = Command::new(cargo_bin("scanfs"));
        command.args([
            "--root",
            dir.path().to_str().unwrap(),
            "--sha256",
            "--format",
            "json",
        ]);
        if enabled {
            command.arg("--phash");
        }
        let out = command.output().unwrap();
        assert!(
            out.status.success(),
            "{}",
            String::from_utf8_lossy(&out.stderr)
        );
        check(&out.stdout, enabled);
    }
}

fn check(bytes: &[u8], enabled: bool) {
    let rows: Vec<Value> = String::from_utf8_lossy(bytes)
        .lines()
        .map(|line| serde_json::from_str(line).unwrap())
        .collect();
    assert_eq!(rows.len(), 15);
    for row in rows {
        let path = row["identity"]["path"].as_str().unwrap();
        let name = Path::new(path).file_name().unwrap().to_str().unwrap();
        assert_eq!(row["schema_version"], 1);
        assert_eq!(row["hashes"]["sha256"].as_str().unwrap().len(), 64);
        if enabled && ["corrupt.png", "truncated.png", "unsupported.jpg"].contains(&name) {
            assert!(row["error"].as_str().unwrap().starts_with("phash:"));
            assert!(row["meta"]["phash"].is_null());
        } else {
            assert!(row["error"].is_null(), "{row}");
            let expected = if enabled {
                Value::from(if name == "constant.png" { 0 } else { GOLDEN })
            } else {
                Value::Null
            };
            assert_eq!(row["meta"]["phash"], expected, "{name}");
        }
    }
}

#[test]
fn budget_errors_remain_records_and_do_not_poison_following_scans() {
    let dir = tempdir().unwrap();
    let fixtures = Path::new(env!("CARGO_MANIFEST_DIR")).join("../crates/phash/tests");
    for name in ["width.png", "height.png", "decoded-budget.png"] {
        fs::copy(fixtures.join("limits").join(name), dir.path().join(name)).unwrap();
    }
    let file = fs::File::create(dir.path().join("encoded-budget.png")).unwrap();
    file.set_len((32 << 20) + 1).unwrap();
    drop(file);
    fs::copy(
        fixtures.join("fixtures/rgb.png"),
        dir.path().join("valid.png"),
    )
    .unwrap();
    for _ in 0..2 {
        let mut bytes = Vec::new();
        scanner::scan(
            &scanner::ScanOptions {
                roots: vec![dir.path().to_owned()],
                phash: true,
                ..Default::default()
            },
            &mut bytes,
        )
        .unwrap();
        check_budget_rows(&bytes);
        let out = Command::new(cargo_bin("scanfs"))
            .args([
                "--root",
                dir.path().to_str().unwrap(),
                "--phash",
                "--format",
                "json",
            ])
            .output()
            .unwrap();
        assert!(out.status.success());
        check_budget_rows(&out.stdout);
    }
}
fn check_budget_rows(bytes: &[u8]) {
    let rows: Vec<Value> = String::from_utf8_lossy(bytes)
        .lines()
        .map(|s| serde_json::from_str(s).unwrap())
        .collect();
    assert_eq!(rows.len(), 5);
    for r in rows {
        let name = Path::new(r["identity"]["path"].as_str().unwrap())
            .file_name()
            .unwrap()
            .to_str()
            .unwrap();
        if name == "valid.png" {
            assert!(r["error"].is_null());
            assert_eq!(r["meta"]["phash"].as_u64(), Some(GOLDEN));
        } else {
            let error = r["error"].as_str().unwrap();
            assert!(
                error == "phash: Image is too large" || error == "phash: Insufficient memory",
                "{error}"
            );
            assert!(r["meta"]["phash"].is_null());
        }
    }
}
