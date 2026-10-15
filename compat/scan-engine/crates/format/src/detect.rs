use file_format::FileFormat;
use std::fs::File;
use std::io::Read;
use std::path::Path;
use thiserror::Error;

const SNIFF_BYTES: usize = 8192;

#[derive(Debug, Clone)]
pub struct FormatSniff {
    pub format_short_name: String,
    pub media_type: String,
    pub extension_matches: bool,
}

#[derive(Debug, Error)]
pub enum FormatError {
    #[error(transparent)]
    Io(#[from] std::io::Error),
}

/// Read the start of `path`, detect format, and compare extension hint when possible.
pub fn sniff_file(path: &Path) -> Result<FormatSniff, FormatError> {
    let mut f = File::open(path)?;
    let mut buf = vec![0u8; SNIFF_BYTES];
    let n = f.read(&mut buf)?;
    buf.truncate(n);

    let kind = FileFormat::from_bytes(&buf);
    let format_short_name = kind.name().to_string();
    let media_type = kind.media_type().to_string();

    let expected = kind.extension();
    let ext_ok = path
        .extension()
        .and_then(|e| e.to_str())
        .map(|ext| expected.eq_ignore_ascii_case(ext))
        .unwrap_or(true);

    Ok(FormatSniff {
        format_short_name,
        media_type,
        extension_matches: ext_ok,
    })
}
