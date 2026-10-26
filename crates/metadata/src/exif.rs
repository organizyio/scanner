use crate::limits::*;
use crate::types::ExifMeta;
use exif::{In, Reader as ExifReader, Tag, Value};
use std::io::BufReader;
use std::path::Path;

/// Best-effort EXIF extraction.
pub fn read_exif(path: &Path) -> Option<ExifMeta> {
    let bytes = read_file(path, MAX_EXIF_CONTAINER)?;
    let mut bufreader = BufReader::new(MetadataInput::new(&bytes, MAX_EXIF_RAW as u64));
    let exifreader = ExifReader::new();
    let exif = exifreader.read_from_container(&mut bufreader).ok()?;
    if exif.buf().len() > MAX_EXIF_RAW || exif.fields().len() > MAX_EXIF_FIELDS {
        return None;
    }
    let mut remaining = MAX_EXIF_VALUES;
    for field in exif.fields() {
        remaining = remaining.checked_sub(value_units(&field.value)?)?;
    }

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

fn value_units(value: &Value) -> Option<usize> {
    Some(match value {
        Value::Byte(v) | Value::Undefined(v, _) => v.len(),
        Value::Ascii(v) => v.iter().try_fold(v.len(), |n, s| n.checked_add(s.len()))?,
        Value::Short(v) => v.len(),
        Value::Long(v) => v.len(),
        Value::Rational(v) => v.len().checked_mul(2)?,
        Value::SByte(v) => v.len(),
        Value::SShort(v) => v.len(),
        Value::SLong(v) => v.len(),
        Value::SRational(v) => v.len().checked_mul(2)?,
        Value::Float(v) => v.len(),
        Value::Double(v) => v.len(),
        Value::Unknown(_, _, _) => 1,
    })
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

#[cfg(test)]
mod resource_tests {
    use super::*;
    use exif::{experimental::Writer, Context, Field};
    fn tiff(fields: &[Field]) -> Vec<u8> {
        let mut writer = Writer::new();
        for f in fields {
            writer.push_field(f);
        }
        let mut bytes = std::io::Cursor::new(Vec::new());
        writer.write(&mut bytes, true).unwrap();
        bytes.into_inner()
    }
    fn extra(tag: u16, value: Value) -> Field {
        Field {
            tag: Tag(Context::Tiff, tag),
            ifd_num: In::PRIMARY,
            value,
        }
    }
    fn read(bytes: &[u8]) -> Option<ExifMeta> {
        let f = tempfile::NamedTempFile::new().unwrap();
        std::fs::write(f.path(), bytes).unwrap();
        read_exif(f.path())
    }
    #[test]
    fn numeric_types_unknown_tags_and_camera_values_are_preserved() {
        let fields = [
            Field {
                tag: Tag::Make,
                ifd_num: In::PRIMARY,
                value: Value::Ascii(vec![b"Synthetic".to_vec()]),
            },
            extra(50000, Value::Long(vec![u32::MAX])),
            extra(50001, Value::Byte(vec![0, 255])),
            extra(
                50002,
                Value::Rational(vec![
                    exif::Rational { num: 1, denom: 0 },
                    exif::Rational { num: 3, denom: 2 },
                ]),
            ),
        ];
        let m = read(&tiff(&fields)).unwrap();
        assert_eq!(m.camera.as_deref(), Some("\"Synthetic\""));
        assert!(m
            .extra
            .values()
            .any(|v| *v == serde_json::json!([4294967295u64])));
        assert!(m.extra.values().any(|v| *v == serde_json::json!([0, 255])));
        assert!(m
            .extra
            .values()
            .any(|v| *v == serde_json::json!([null, 1.5])));
    }
    #[test]
    fn fields_and_values_are_declined_whole_instead_of_truncated() {
        let exact = tiff(&[extra(50000, Value::Undefined(vec![7; MAX_EXIF_VALUES], 0))]);
        assert!(read(&exact).is_some());
        let over = tiff(&[extra(
            50000,
            Value::Undefined(vec![7; MAX_EXIF_VALUES + 1], 0),
        )]);
        assert!(read(&over).is_none());
        let fields: Vec<_> = (0..MAX_EXIF_FIELDS + 1)
            .map(|n| extra(10000 + n as u16, Value::Short(vec![1])))
            .collect();
        assert!(read(&tiff(&fields)).is_none());
        assert!(read(&tiff(&fields[..MAX_EXIF_FIELDS])).is_some());
        assert!(read(&exact).is_some());
    }
    #[test]
    fn raw_data_and_forged_value_counts_cannot_escape_parser_read_budget() {
        let data = tiff(&[extra(50000, Value::Undefined(vec![7; MAX_EXIF_RAW + 1], 0))]);
        assert!(read(&data).is_none());
        let mut forged = tiff(&[extra(50000, Value::Long(vec![1, 2]))]);
        forged[14..18].copy_from_slice(&u32::MAX.to_le_bytes());
        assert!(read(&forged).is_none());
        for end in 0..24 {
            assert!(read(&forged[..end]).is_none());
        }
    }
}
