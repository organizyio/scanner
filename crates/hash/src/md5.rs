use md5::{Digest, Md5};
use std::fs::File;
use std::io::{self, Read};
use std::path::Path;

/// Full-file MD5 as lowercase hex.
pub fn hash_file_md5_hex(path: &Path) -> io::Result<String> {
    let mut file = File::open(path)?;
    let mut hasher = Md5::new();
    let mut buf = [0u8; 256 * 1024];
    loop {
        let n = file.read(&mut buf)?;
        if n == 0 {
            break;
        }
        hasher.update(&buf[..n]);
    }
    Ok(format!("{:x}", hasher.finalize()))
}
