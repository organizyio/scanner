use metadata::{read_docx_core, read_exif, read_pdf_info};
#[test]
fn oversized_container_is_declined_without_parsing_and_next_file_recovers() {
    let file = tempfile::NamedTempFile::new().unwrap();
    file.as_file().set_len((64 << 20) + 1).unwrap();
    assert!(read_docx_core(file.path()).is_none());
    assert!(read_pdf_info(file.path()).is_none());
    assert!(read_exif(file.path()).is_none());
    let root = std::path::Path::new(env!("CARGO_MANIFEST_DIR")).join("tests/fixtures");
    assert!(read_docx_core(&root.join("valid.docx")).is_some());
    assert_eq!(
        read_pdf_info(&root.join("valid.pdf")).unwrap().page_count,
        Some(1)
    );
    assert!(read_exif(&root.join("valid.tiff")).is_some());
}
