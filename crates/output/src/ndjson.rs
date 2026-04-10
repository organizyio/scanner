//! One JSON object per line (NDJSON).

use crate::FileRecord;
use std::io::{self, Write};

/// Serialize `record` as a single JSON line (no trailing newline).
pub fn write_record<W: Write>(writer: &mut W, record: &FileRecord) -> io::Result<()> {
    serde_json::to_writer(writer, record).map_err(io::Error::other)
}

/// Same as [`write_record`] then append `\n`.
pub fn write_record_line<W: Write>(writer: &mut W, record: &FileRecord) -> io::Result<()> {
    write_record(writer, record)?;
    writer.write_all(b"\n")
}
