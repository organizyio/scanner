use std::fs::File;
use std::io::{self, Read};
use std::path::Path;
use xxhash_rust::xxh64::Xxh64;

/// Full-file xxHash64 as lowercase hex.
pub fn hash_file_xxhash64_hex(path: &Path) -> io::Result<String> {
    let mut file = File::open(path)?;
    let mut hasher = Xxh64::new(0);
    let mut buf = [0u8; 256 * 1024];
    loop {
        let n = file.read(&mut buf)?;
        if n == 0 {
            break;
        }
        hasher.update(&buf[..n]);
    }
    Ok(format!("{:016x}", hasher.digest()))
}
