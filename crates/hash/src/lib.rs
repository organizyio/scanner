//! File hashing: xxHash64, SHA-256, partial reads.

mod md5;
mod partial;
mod sha256;
mod xxhash;

pub use md5::hash_file_md5_hex;
pub use partial::{partial_digest, PartialOptions};
pub use sha256::hash_file_sha256_hex;
pub use xxhash::hash_file_xxhash64_hex;
