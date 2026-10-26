//! 64-bit perceptual hash (raw value for downstream use).

mod phash;
mod png;

pub use phash::{phash_u64, PhashError};
