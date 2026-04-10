use assert_cmd::cargo::cargo_bin;
use serde_json::Value;
use std::fs;
use std::process::Command;
use tempfile::tempdir;

#[test]
fn json_output_includes_normalized_fields() {
    let dir = tempdir().unwrap();
    let root = dir.path().join("root");
    fs::create_dir_all(root.join("sub")).unwrap();
    let file = root.join("sub").join("a.txt");
    fs::write(&file, b"abc").unwrap();

    let out = run_scanfs(&["--root", root.to_str().unwrap(), "--format", "json"])
        .env("SCAN_FS_SOURCE", "test-source")
        .output()
        .unwrap();
    assert!(
        out.status.success(),
        "stderr: {}",
        String::from_utf8_lossy(&out.stderr)
    );
    let stdout = String::from_utf8(out.stdout).unwrap();
    let first = stdout.lines().next().unwrap();
    let v: Value = serde_json::from_str(first).unwrap();

    assert_eq!(v["source"], Value::String("test-source".to_string()));
    assert_eq!(v["device"], Value::String(root.display().to_string()));
    assert_eq!(
        v["directory"],
        Value::String(file.parent().unwrap().display().to_string())
    );
    assert_eq!(v["name"], Value::String("a.txt".to_string()));
    assert_eq!(v["extension"], Value::String(".txt".to_string()));
    assert_eq!(v["schema_version"], Value::from(1));
    assert_eq!(
        v["identity"]["path"],
        Value::String(file.display().to_string())
    );
}

#[test]
fn multi_root_device_uses_first_root_for_all_rows() {
    let dir = tempdir().unwrap();
    let root_a = dir.path().join("a");
    let root_b = dir.path().join("b");
    fs::create_dir_all(&root_a).unwrap();
    fs::create_dir_all(&root_b).unwrap();
    fs::write(root_a.join("one.txt"), b"one").unwrap();
    fs::write(root_b.join("two.txt"), b"two").unwrap();

    let out = run_scanfs(&[
        "--root",
        root_a.to_str().unwrap(),
        "--root",
        root_b.to_str().unwrap(),
        "--format",
        "json",
    ])
    .env("SCAN_FS_SOURCE", "s")
    .output()
    .unwrap();
    assert!(
        out.status.success(),
        "stderr: {}",
        String::from_utf8_lossy(&out.stderr)
    );
    let stdout = String::from_utf8(out.stdout).unwrap();
    let rows: Vec<Value> = stdout
        .lines()
        .map(|line| serde_json::from_str::<Value>(line).unwrap())
        .collect();
    assert!(rows.len() >= 2);
    for row in rows {
        assert_eq!(row["device"], Value::String(root_a.display().to_string()));
    }
}

#[test]
fn deep_profile_enables_md5_and_sha256_defaults() {
    let dir = tempdir().unwrap();
    let root = dir.path().join("root");
    fs::create_dir_all(&root).unwrap();
    fs::write(root.join("a.txt"), b"abc").unwrap();
    let out = run_scanfs(&[
        "--root",
        root.to_str().unwrap(),
        "--format",
        "json",
        "--profile",
        "deep",
    ])
    .output()
    .unwrap();
    assert!(
        out.status.success(),
        "stderr: {}",
        String::from_utf8_lossy(&out.stderr)
    );
    let stdout = String::from_utf8(out.stdout).unwrap();
    let first = stdout.lines().next().unwrap();
    let v: Value = serde_json::from_str(first).unwrap();
    assert!(v["hashes"]["md5"].as_str().is_some());
    assert!(v["hashes"]["sha256"].as_str().is_some());
}

fn run_scanfs(args: &[&str]) -> Command {
    let mut cmd = Command::new(cargo_bin("scanfs"));
    cmd.args(args);
    cmd
}
