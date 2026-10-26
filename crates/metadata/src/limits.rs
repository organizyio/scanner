//! Preferred metadata budgets; these are not total process-memory or CPU bounds.
use std::{
    fs::File,
    io::{self, Read, Seek, SeekFrom},
    path::Path,
};
pub(crate) const MAX_DOCUMENT: u64 = 64 * 1024 * 1024;
pub(crate) const MAX_EXIF_CONTAINER: u64 = 32 * 1024 * 1024;
pub(crate) const MAX_CORE_XML: u64 = 2 * 1024 * 1024;
pub(crate) const MAX_XML_NODES: u32 = 50_000;
pub(crate) const MAX_ZIP_READ: u64 = 8 * 1024 * 1024;
pub(crate) const MAX_ZIP_ENTRIES: usize = 4096;
pub(crate) const MAX_PDF_STREAM: usize = 8 * 1024 * 1024;
pub(crate) const MAX_PDF_OBJECTS: usize = 100_000;
pub(crate) const MAX_PDF_DEPTH: usize = 256;
pub(crate) const MAX_EXIF_RAW: usize = 2 * 1024 * 1024;
pub(crate) const MAX_EXIF_FIELDS: usize = 4096;
pub(crate) const MAX_EXIF_VALUES: usize = 262_144;

pub(crate) fn read_file(path: &Path, max: u64) -> Option<Vec<u8>> {
    let file = File::open(path).ok()?;
    if file.metadata().ok()?.len() > max {
        return None;
    }
    read(file, max)
}
pub(crate) fn read(reader: impl Read, max: u64) -> Option<Vec<u8>> {
    let mut bytes = Vec::new();
    reader
        .take(max.checked_add(1)?)
        .read_to_end(&mut bytes)
        .ok()?;
    (bytes.len() as u64 <= max).then_some(bytes)
}
/// Cumulative parser reads are capped across seeks and repeated probes.
pub(crate) struct MetadataInput<'a> {
    cursor: io::Cursor<&'a [u8]>,
    remaining: u64,
}
impl<'a> MetadataInput<'a> {
    pub(crate) fn new(data: &'a [u8], budget: u64) -> Self {
        Self {
            cursor: io::Cursor::new(data),
            remaining: budget,
        }
    }
}
impl Read for MetadataInput<'_> {
    fn read(&mut self, buf: &mut [u8]) -> io::Result<usize> {
        if buf.is_empty() {
            return Ok(0);
        }
        if self.cursor.position() == self.cursor.get_ref().len() as u64 {
            return Ok(0);
        }
        if self.remaining == 0 {
            return Err(io::Error::other("metadata read budget exceeded"));
        }
        let limit = buf.len().min(self.remaining as usize);
        let n = self.cursor.read(&mut buf[..limit])?;
        self.remaining -= n as u64;
        Ok(n)
    }
}
impl Seek for MetadataInput<'_> {
    fn seek(&mut self, from: SeekFrom) -> io::Result<u64> {
        let position = match from {
            SeekFrom::Start(n) => i128::from(n),
            SeekFrom::Current(n) => i128::from(self.cursor.position()) + i128::from(n),
            SeekFrom::End(n) => self.cursor.get_ref().len() as i128 + i128::from(n),
        };
        if position < 0 || position > self.cursor.get_ref().len() as i128 {
            return Err(io::Error::other("metadata seek outside input"));
        }
        self.cursor.seek(SeekFrom::Start(position as u64))
    }
}
#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn encoded_limit_catches_growth_and_retains_inclusive_boundary() {
        assert_eq!(read(&b"1234"[..], 4).unwrap(), b"1234");
        assert!(read(std::io::repeat(0), 4).is_none());
    }
    #[test]
    fn zip_read_budget_is_cumulative_and_seeks_cannot_escape_buffer() {
        let mut input = MetadataInput::new(b"1234", 4);
        input.remaining = 4;
        let mut b = [0; 4];
        input.read_exact(&mut b).unwrap();
        input.seek(SeekFrom::Start(0)).unwrap();
        assert!(input.read(&mut b).is_err());
        assert!(input.seek(SeekFrom::Start(5)).is_err());
        assert!(input.seek(SeekFrom::End(-5)).is_err());
    }
}
