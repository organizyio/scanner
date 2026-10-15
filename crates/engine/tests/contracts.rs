use engine::{scan, ScanOptions};
use serde_json::Value;
use std::fs;
use tempfile::tempdir;

#[test]
fn preferred_record_matches_pre_change_golden() {
    let dir = tempdir().unwrap();
    fs::write(dir.path().join("hello.txt"), b"abc").unwrap();
    let opts = ScanOptions {
        roots: vec![dir.path().to_owned()],
        ..Default::default()
    };
    let mut bytes = Vec::new();
    scan(&opts, &mut bytes).unwrap();
    let mut actual: Value = serde_json::from_slice(&bytes).unwrap();
    // Copied from the immutable phase-1 baseline-contracts.json preferred
    // hello.txt record, kept in-package for cargo package verification.
    let mut expected: Value =
        serde_json::from_str(include_str!("fixtures/preferred-record.json")).unwrap();
    for key in ["source", "device", "directory", "name", "extension"] {
        expected.as_object_mut().unwrap().remove(key);
    }
    actual["identity"]["path"] = expected["identity"]["path"].clone();
    for key in ["modified_at", "accessed_at", "created_at"] {
        // Availability of native birth/access timestamps varies by platform.
        if actual["identity"].get(key).is_some() {
            actual["identity"][key] = Value::from("<timestamp>");
        } else {
            expected["identity"].as_object_mut().unwrap().remove(key);
        }
    }
    for key in ["inode", "device_id"] {
        if actual["identity"].get(key).is_some() {
            actual["identity"][key] = Value::from(0);
        } else {
            expected["identity"].as_object_mut().unwrap().remove(key);
        }
    }
    assert_eq!(actual, expected);
}
