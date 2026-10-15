//! Public scan API (`Scanner`, `ScanOptions`, `scan`).

mod error;
mod scanner;

pub use error::ScanError;
pub use scanner::{scan, ScanOptions, Scanner, XxhashMode};
