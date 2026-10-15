use crate::types::ExifMeta;
use exif::{In, Reader as ExifReader, Tag};
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

    if info.camera.is_none() && info.lens.is_none() {
        None
    } else {
        Some(info)
    }
}
