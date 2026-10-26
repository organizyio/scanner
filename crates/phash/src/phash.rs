use img_hash::image;
use img_hash::{HashAlg, HasherConfig};
use std::path::Path;
use thiserror::Error;

#[derive(Debug, Error)]
pub enum PhashError {
    #[error(transparent)]
    Image(#[from] image::ImageError),
    #[error("hash length unexpected")]
    BadLength,
}

/// Perceptual hash as a single `u64` (first 8 bytes of 8×8 gradient hash).
///
/// PNG input is limited to 32 MiB encoded bytes, 16,384 pixels per dimension
/// and 64 MiB decoded samples. Resource failures remain `PhashError::Image`.
/// These limits do not bound process RSS or preempt active I/O/decoding.
pub fn phash_u64(path: &Path) -> Result<u64, PhashError> {
    let img = if matches!(
        image::ImageFormat::from_path(path),
        Ok(image::ImageFormat::Png)
    ) {
        crate::png::open(path)?
    } else {
        image::open(path)?
    };
    let config = HasherConfig::new()
        .hash_size(8, 8)
        .hash_alg(HashAlg::Gradient);
    let hasher = config.to_hasher();
    let hash = hasher.hash_image(&img);
    let bytes = hash.as_bytes();
    if bytes.len() < 8 {
        return Err(PhashError::BadLength);
    }
    let mut v: u64 = 0;
    for &b in &bytes[..8] {
        v = (v << 8) | u64::from(b);
    }
    Ok(v)
}
