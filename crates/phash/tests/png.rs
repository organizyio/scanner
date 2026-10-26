use phash::{phash_u64, PhashError};
use std::path::Path;

#[test]
fn known_pixels_have_exact_gradient_without_codec_oracle() {
    // The independently generated 9x8 fixtures need no resize: alternating
    // ascending/descending rows yield FF/00. Constant pixels yield zero.
    let root = Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures");
    for name in [
        "rgb.png",
        "rgba.png",
        "gray.png",
        "gray-alpha.png",
        "indexed.png",
        "gray16.png",
        "rgb16.png",
        "adam7.png",
    ] {
        assert_eq!(
            phash_u64(&root.join(name)).unwrap(),
            0xff00ff00ff00ff00,
            "{name}"
        );
    }
    assert_eq!(phash_u64(&root.join("constant.png")).unwrap(), 0);
    assert!(matches!(
        phash_u64(&root.join("missing.png")),
        Err(PhashError::Image(_))
    ));
}

#[test]
fn mixed_gradient_golden_is_derived_from_pixels_without_png_decoding() {
    use img_hash::{
        image::{DynamicImage, Rgb, RgbImage},
        HashAlg, HasherConfig,
    };
    let pixels = RgbImage::from_fn(1024, 768, |x, y| {
        Rgb([((x + y) % 256) as u8, (x % 256) as u8, (y % 256) as u8])
    });
    let hash = HasherConfig::new()
        .hash_size(8, 8)
        .hash_alg(HashAlg::Gradient)
        .to_hasher()
        .hash_image(&DynamicImage::ImageRgb8(pixels));
    assert_eq!(
        u64::from_be_bytes(hash.as_bytes().try_into().unwrap()),
        13089050091743192493
    );
}
