//! Stable public API for filesystem scanning (`ScanOptions`, `scan`, `FileRecord`, …).
//!
//! In-repo crates under `scanner/crates/*` are implementation details; depend on this
//! package for integration (e.g. workers, tools).

pub use engine::{
    scan, scan_with_callbacks, ScanError, ScanOptions, ScanProgress, Scanner, XxhashMode,
};
pub use hash::PartialOptions;
pub use output::{FileRecord, IdentityInfo};
pub use walker::{FilterOptions, WalkMode};
