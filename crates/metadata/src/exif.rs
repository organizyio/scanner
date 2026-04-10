use crate::types::ExifMeta;
use exif::{In, Reader as ExifReader, Tag, Value};
use std::fs::File;
use std::io::BufReader;
use std::path::Path;

/// Best-effort EXIF extraction.
pub fn read_exif(path: &Path) -> Option<ExifMeta> {
    let file = File::open(path).ok()?;
    let mut bufreader = BufReader::new(&file);
    let exifreader = ExifReader::new();
    let exif = exifreader.read_from_container(&mut bufreader).ok()?;

    let mut info = ExifMeta::default();

    let make = exif
        .get_field(Tag::Make, In::PRIMARY)
        .map(|f| f.display_value().to_string());
    let model = exif
        .get_field(Tag::Model, In::PRIMARY)
        .map(|f| f.display_value().to_string());
    info.camera = match (make, model) {
        (Some(m), Some(md)) if !m.is_empty() && !md.is_empty() => Some(format!("{m} {md}")),
        (Some(m), _) if !m.is_empty() => Some(m),
        (_, Some(md)) if !md.is_empty() => Some(md),
        _ => None,
    };

    if let Some(f) = exif.get_field(Tag::LensModel, In::PRIMARY) {
        let s = f.display_value().to_string();
        if !s.is_empty() {
            info.lens = Some(s);
        }
    }

    let lat_ref = exif
        .get_field(Tag::GPSLatitudeRef, In::PRIMARY)
        .map(|f| f.display_value().to_string());
    let lon_ref = exif
        .get_field(Tag::GPSLongitudeRef, In::PRIMARY)
        .map(|f| f.display_value().to_string());
    if let Some(f) = exif.get_field(Tag::GPSLatitude, In::PRIMARY) {
        info.gps_lat = parse_gps_decimal(f.display_value().to_string(), lat_ref.as_deref());
    }
    if let Some(f) = exif.get_field(Tag::GPSLongitude, In::PRIMARY) {
        info.gps_lon = parse_gps_decimal(f.display_value().to_string(), lon_ref.as_deref());
    }

    for field in exif.fields() {
        if is_known_tag(field.tag) {
            continue;
        }
        let key = format!("{:?}", field.tag);
        info.extra.insert(key, exif_value_to_json(&field.value));
    }

    if info.camera.is_none()
        && info.lens.is_none()
        && info.gps_lat.is_none()
        && info.gps_lon.is_none()
        && info.extra.is_empty()
    {
        None
    } else {
        Some(info)
    }
}

fn is_known_tag(tag: Tag) -> bool {
    matches!(
        tag,
        Tag::Make
            | Tag::Model
            | Tag::LensModel
            | Tag::GPSLatitude
            | Tag::GPSLongitude
            | Tag::GPSLatitudeRef
            | Tag::GPSLongitudeRef
    )
}

fn parse_gps_decimal(text: String, reference: Option<&str>) -> Option<f64> {
    let filtered = text
        .replace("deg", " ")
        .replace("min", " ")
        .replace("sec", " ")
        .replace(['"', '\''], " ");
    let mut nums = filtered
        .split_whitespace()
        .filter_map(|p| p.parse::<f64>().ok());
    let deg = nums.next()?;
    let min = nums.next().unwrap_or(0.0);
    let sec = nums.next().unwrap_or(0.0);
    let mut decimal = deg + (min / 60.0) + (sec / 3600.0);
    if let Some(reference) = reference {
        let upper = reference.trim().to_ascii_uppercase();
        if upper.contains('S') || upper.contains('W') {
            decimal *= -1.0;
        }
    }
    Some(decimal)
}

fn exif_value_to_json(value: &Value) -> serde_json::Value {
    match value {
        Value::Byte(v) => serde_json::json!(v),
        Value::Ascii(v) => serde_json::json!(v
            .iter()
            .map(|s| String::from_utf8_lossy(s).to_string())
            .collect::<Vec<_>>()),
        Value::Short(v) => serde_json::json!(v),
        Value::Long(v) => serde_json::json!(v),
        Value::Rational(v) => serde_json::json!(v
            .iter()
            .map(|r| {
                if r.denom == 0 {
                    serde_json::Value::Null
                } else {
                    serde_json::json!(r.num as f64 / r.denom as f64)
                }
            })
            .collect::<Vec<_>>()),
        Value::SByte(v) => serde_json::json!(v),
        Value::Undefined(v, _) => serde_json::json!(v),
        Value::SShort(v) => serde_json::json!(v),
        Value::SLong(v) => serde_json::json!(v),
        Value::SRational(v) => serde_json::json!(v
            .iter()
            .map(|r| {
                if r.denom == 0 {
                    serde_json::Value::Null
                } else {
                    serde_json::json!(r.num as f64 / r.denom as f64)
                }
            })
            .collect::<Vec<_>>()),
        Value::Float(v) => serde_json::json!(v),
        Value::Double(v) => serde_json::json!(v),
        Value::Unknown(_, _, _) => serde_json::Value::Null,
    }
}

#[cfg(test)]
mod tests {
    use super::parse_gps_decimal;

    #[test]
    fn gps_ref_south_and_west_are_negative() {
        let lat = parse_gps_decimal("40 deg 26 min 46 sec".to_string(), Some("S")).unwrap();
        let lon = parse_gps_decimal("79 deg 58 min 56 sec".to_string(), Some("W")).unwrap();
        assert!(lat < 0.0);
        assert!(lon < 0.0);
    }

    #[test]
    fn gps_ref_north_and_east_stay_positive() {
        let lat = parse_gps_decimal("40 deg 26 min 46 sec".to_string(), Some("N")).unwrap();
        let lon = parse_gps_decimal("79 deg 58 min 56 sec".to_string(), Some("E")).unwrap();
        assert!(lat > 0.0);
        assert!(lon > 0.0);
    }
}
