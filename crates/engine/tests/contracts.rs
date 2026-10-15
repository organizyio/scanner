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
    let baseline: Value = serde_json::from_str(include_str!(
        "../../../docs/follow-up/baseline-contracts.json"
    ))
    .unwrap();
    let mut expected = baseline["preferred"]
        .as_array()
        .unwrap()
        .iter()
        .find(|v| v["name"] == "hello.txt")
        .unwrap()
        .clone();
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
