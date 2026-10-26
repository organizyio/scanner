use img_hash::image::{error::LimitErrorKind, ImageError};
use phash::{phash_u64, PhashError};
use std::path::Path;
#[test]
fn oversized_headers_are_rejected_before_the_absent_pixel_payload() {
    for (name, expected) in [
        ("width.png", LimitErrorKind::DimensionError),
        ("height.png", LimitErrorKind::DimensionError),
        ("decoded-budget.png", LimitErrorKind::InsufficientMemory),
    ] {
        let path = Path::new(env!("CARGO_MANIFEST_DIR"))
            .join("tests/limits")
            .join(name);
        match phash_u64(&path).unwrap_err() {
            PhashError::Image(ImageError::Limits(error)) => {
                assert_eq!(error.kind(), expected, "{name}")
            }
            error => panic!("expected an early limit error, got {error}: {name}"),
        }
    }
}
