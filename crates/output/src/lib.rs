//! `FileRecord` and NDJSON streaming output.

mod ndjson;
mod record;

pub use ndjson::{write_record, write_record_line};
pub use record::{
    utc_from_system_time, AudioInfo, DocxCore, ExifInfo, FileRecord, FormatInfo, HashesInfo,
    IdentityInfo, MetaInfo, PdfInfo, VideoInfo,
};
