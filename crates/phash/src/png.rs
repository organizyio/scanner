//! Budgeted PNG decoding; hashing and image conversion types remain unchanged.
use img_hash::image::{
    error::{DecodingError, LimitError, LimitErrorKind},
    DynamicImage, ImageBuffer, ImageError, ImageFormat,
};
use std::{
    fs::File,
    io::{BufReader, Cursor, Read},
    path::Path,
};

pub(super) const MAX_ENCODED: u64 = 32 * 1024 * 1024;
pub(super) const MAX_DECODED: usize = 64 * 1024 * 1024;
pub(super) const MAX_SIDE: u32 = 16_384;

fn limit(kind: LimitErrorKind) -> ImageError {
    ImageError::Limits(LimitError::from_kind(kind))
}
fn memory_limit() -> ImageError {
    limit(LimitErrorKind::InsufficientMemory)
}
fn png_error(err: png::DecodingError) -> ImageError {
    match err {
        png::DecodingError::IoError(err) => ImageError::IoError(err),
        png::DecodingError::LimitsExceeded => memory_limit(),
        err => ImageError::Decoding(DecodingError::new(ImageFormat::Png.into(), err)),
    }
}
fn dimensions(width: u32, height: u32) -> Result<(), ImageError> {
    if width == 0 || height == 0 || width > MAX_SIDE || height > MAX_SIDE {
        return Err(limit(LimitErrorKind::DimensionError));
    }
    Ok(())
}
fn buffer_size(size: Option<usize>) -> Result<usize, ImageError> {
    size.filter(|&n| n <= MAX_DECODED).ok_or_else(memory_limit)
}

pub(super) fn open(path: &Path) -> Result<DynamicImage, ImageError> {
    let file = File::open(path)?;
    if file.metadata()?.len() > MAX_ENCODED {
        return Err(memory_limit());
    }
    let encoded = read_encoded(BufReader::new(file), MAX_ENCODED)?;
    decode(&encoded)
}

fn read_encoded(reader: impl Read, max: u64) -> Result<Vec<u8>, ImageError> {
    // The read cap catches growth after metadata inspection. Decode these same
    // bytes, avoiding a reopen/header-to-payload identity race.
    let mut encoded = Vec::new();
    reader.take(max + 1).read_to_end(&mut encoded)?;
    if encoded.len() as u64 > max {
        return Err(memory_limit());
    }
    Ok(encoded)
}

fn decode(encoded: &[u8]) -> Result<DynamicImage, ImageError> {
    let mut decoder =
        png::Decoder::new_with_limits(Cursor::new(encoded), png::Limits { bytes: MAX_DECODED });
    // Text/ICC are irrelevant to the existing sample-based luminance hash.
    decoder.set_ignore_text_chunk(true);
    decoder.set_ignore_iccp_chunk(true);
    decoder.set_transformations(png::Transformations::EXPAND);
    let header = decoder.read_header_info().map_err(png_error)?;
    dimensions(header.width, header.height)?;
    let mut reader = decoder.read_info().map_err(png_error)?;
    let size = buffer_size(reader.output_buffer_size())?;
    let mut bytes = Vec::new();
    bytes.try_reserve_exact(size).map_err(|_| memory_limit())?;
    bytes.resize(size, 0);
    let output = reader.next_frame(&mut bytes).map_err(png_error)?;
    dimensions(output.width, output.height)?;
    let used = buffer_size(Some(output.buffer_size()))?;
    bytes.truncate(used);
    let (width, height) = (output.width, output.height);
    macro_rules! image {
        ($variant:ident, $data:expr) => {
            ImageBuffer::from_raw(width, height, $data)
                .map(DynamicImage::$variant)
                .ok_or_else(memory_limit)
        };
    }
    if output.bit_depth == png::BitDepth::Eight {
        match output.color_type {
            png::ColorType::Grayscale => image!(ImageLuma8, bytes),
            png::ColorType::GrayscaleAlpha => image!(ImageLumaA8, bytes),
            png::ColorType::Rgb => image!(ImageRgb8, bytes),
            png::ColorType::Rgba => image!(ImageRgba8, bytes),
            _ => Err(ImageError::Decoding(DecodingError::from_format_hint(
                ImageFormat::Png.into(),
            ))),
        }
    } else if output.bit_depth == png::BitDepth::Sixteen {
        let mut samples = Vec::new();
        samples
            .try_reserve_exact(used / 2)
            .map_err(|_| memory_limit())?;
        samples.extend(
            bytes
                .chunks_exact(2)
                .map(|b| u16::from_be_bytes([b[0], b[1]])),
        );
        match output.color_type {
            png::ColorType::Grayscale => image!(ImageLuma16, samples),
            png::ColorType::GrayscaleAlpha => image!(ImageLumaA16, samples),
            png::ColorType::Rgb => image!(ImageRgb16, samples),
            png::ColorType::Rgba => image!(ImageRgba16, samples),
            _ => Err(ImageError::Decoding(DecodingError::from_format_hint(
                ImageFormat::Png.into(),
            ))),
        }
    } else {
        Err(ImageError::Decoding(DecodingError::from_format_hint(
            ImageFormat::Png.into(),
        )))
    }
}

#[cfg(test)]
mod tests {
    use super::*;
    #[test]
    fn encoded_stream_growth_is_capped_without_trusting_metadata() {
        assert_eq!(read_encoded(&b"12345678"[..], 8).unwrap(), b"12345678");
        assert!(matches!(
            read_encoded(std::io::repeat(0), 8),
            Err(ImageError::Limits(_))
        ));
    }
    #[test]
    fn exact_budget_edges_are_checked_without_allocating_pixel_buffers() {
        assert!(dimensions(MAX_SIDE, MAX_SIDE).is_ok());
        for (w, h) in [
            (0, 1),
            (1, 0),
            (MAX_SIDE + 1, 1),
            (1, MAX_SIDE + 1),
            (u32::MAX, u32::MAX),
        ] {
            assert!(dimensions(w, h).is_err());
        }
        assert_eq!(buffer_size(Some(MAX_DECODED)).unwrap(), MAX_DECODED);
        assert!(buffer_size(Some(MAX_DECODED + 1)).is_err());
        assert!(buffer_size(None).is_err());
    }
}

#[cfg(test)]
mod malformed_tests {
    use super::decode;
    #[test]
    fn truncated_and_mutated_synthetic_inputs_do_not_panic() {
        let valid = include_bytes!("../tests/fixtures/rgb.png");
        for end in 0..valid.len() {
            assert!(std::panic::catch_unwind(|| decode(&valid[..end])).is_ok());
        }
        for index in 0..valid.len() {
            let mut changed = valid.to_vec();
            changed[index] ^= 0xff;
            assert!(std::panic::catch_unwind(|| decode(&changed)).is_ok());
        }
    }
}
