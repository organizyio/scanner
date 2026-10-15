//! Magic-byte sniffing via the `file-format` crate.

mod detect;

pub use detect::{sniff_file, FormatSniff};
