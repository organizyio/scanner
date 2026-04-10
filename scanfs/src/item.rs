use crate::normalize::Normalizer;
use scanner::FileRecord;
use serde::Serialize;

#[derive(Debug, Clone, Serialize)]
pub struct Item {
    #[serde(flatten)]
    pub record: FileRecord,
    pub source: String,
    pub device: String,
    pub directory: String,
    pub name: String,
    pub extension: String,
}

impl Item {
    pub fn from_record(record: FileRecord, normalizer: &Normalizer) -> Self {
        let (directory, name, extension) = normalizer.path_parts(&record.identity.path);
        Self {
            record,
            source: normalizer.source.clone(),
            device: normalizer.device.clone(),
            directory,
            name,
            extension,
        }
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    use chrono::{TimeZone, Utc};
    use scanner::IdentityInfo;

    #[test]
    fn item_json_roundtrip_keeps_timestamp_fields() {
        let normalizer = Normalizer {
            source: "s".into(),
            device: "d".into(),
        };
        let rec = FileRecord {
            schema_version: 1,
            identity: IdentityInfo {
                path: "/tmp/t.txt".into(),
                size: 10,
                modified_at: Some(Utc.with_ymd_and_hms(2026, 4, 6, 12, 0, 0).unwrap()),
                ..Default::default()
            },
            ..Default::default()
        };
        let item = Item::from_record(rec, &normalizer);
        let json = serde_json::to_string(&item).unwrap();
        let v: serde_json::Value = serde_json::from_str(&json).unwrap();
        assert!(v
            .get("identity")
            .and_then(|i| i.get("modified_at"))
            .is_some());
        assert_eq!(v["source"], serde_json::json!("s"));
    }
}
