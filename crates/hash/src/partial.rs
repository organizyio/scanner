use std::fs::File;
use std::io::{self, Read, Seek, SeekFrom};
use std::path::Path;
use xxhash_rust::xxh64::Xxh64;

/// Tiered read: first `head` bytes + last `tail` bytes (if file larger than `head + tail`).
#[derive(Debug, Clone)]
pub struct PartialOptions {
    pub head: usize,
    pub tail: usize,
}

impl Default for PartialOptions {
    fn default() -> Self {
        Self {
            head: 64 * 1024,
            tail: 64 * 1024,
        }
    }
}

/// xxHash64 over partial content (head + tail strategy), lowercase hex.
pub fn partial_digest(path: &Path, opts: &PartialOptions) -> io::Result<String> {
    let mut file = File::open(path)?;
    let len = file.metadata()?.len() as usize;
    let mut hasher = Xxh64::new(0);

    if len <= opts.head + opts.tail {
        let mut buf = Vec::new();
        file.read_to_end(&mut buf)?;
        hasher.update(&buf);
    } else {
        let mut head = vec![0u8; opts.head];
        file.read_exact(&mut head)?;
        hasher.update(&head);

        let tail_start = len.saturating_sub(opts.tail);
        file.seek(SeekFrom::Start(tail_start as u64))?;
        let mut tail = vec![0u8; opts.tail];
        file.read_exact(&mut tail)?;
        hasher.update(&tail);
    }

    Ok(format!("{:016x}", hasher.digest()))
}
