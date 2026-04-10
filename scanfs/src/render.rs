use crate::cli::OutputFormat;
use crate::item::Item;
use std::io::{self, Write};

pub fn render_item<W: Write>(writer: &mut W, mode: OutputFormat, item: &Item) -> io::Result<()> {
    match mode {
        OutputFormat::Plain => {
            writeln!(writer, "{}", item.record.identity.path)?;
        }
        OutputFormat::Json => {
            serde_json::to_writer(&mut *writer, item).map_err(io::Error::other)?;
            writer.write_all(b"\n")?;
        }
        OutputFormat::Table => {
            writeln!(
                writer,
                "{} | {} | {} | {} | {} | {} | {} | {}",
                item.record.identity.path,
                item.directory,
                item.name,
                item.extension,
                item.record.identity.size,
                item.record
                    .hashes
                    .as_ref()
                    .and_then(|h| h.xxhash64.as_deref())
                    .unwrap_or(""),
                item.record
                    .hashes
                    .as_ref()
                    .and_then(|h| h.sha256.as_deref())
                    .unwrap_or(""),
                item.record
                    .format
                    .as_ref()
                    .and_then(|f| f.mime.as_deref())
                    .unwrap_or("")
            )?;
        }
    }
    Ok(())
}
