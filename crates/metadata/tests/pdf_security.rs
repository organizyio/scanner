use metadata::read_pdf_info;

fn pdf(depth: usize) -> Vec<u8> {
    let mut bytes = b"%PDF-1.5\n".to_vec();
    let mut offsets = Vec::new();
    let nested = format!("{}0{}", "[".repeat(depth), "]".repeat(depth));
    for (index, object) in [
        format!("<< /Type /Catalog /Pages 2 0 R /X {nested} >>"),
        "<< /Type /Pages /Kids [3 0 R] /Count 1 >>".into(),
        "<< /Type /Page /Parent 2 0 R /MediaBox [0 0 100 100] >>".into(),
    ]
    .iter()
    .enumerate()
    {
        offsets.push(bytes.len());
        bytes.extend(format!("{} 0 obj\n{}\nendobj\n", index + 1, object).as_bytes());
    }
    let xref = bytes.len();
    bytes.extend(b"xref\n0 4\n0000000000 65535 f \n");
    for offset in offsets {
        bytes.extend(format!("{offset:010} 00000 n \n").as_bytes());
    }
    bytes.extend(
        format!("trailer\n<< /Size 4 /Root 1 0 R >>\nstartxref\n{xref}\n%%EOF\n").as_bytes(),
    );
    bytes
}

#[test]
fn normal_pdf_page_count_is_preserved() {
    let file = tempfile::NamedTempFile::new().unwrap();
    std::fs::write(file.path(), pdf(1)).unwrap();
    assert_eq!(read_pdf_info(file.path()).unwrap().page_count, Some(1));
}

#[test]
fn deeply_nested_pdf_is_rejected_without_process_abort() {
    let file = tempfile::NamedTempFile::new().unwrap();
    std::fs::write(file.path(), pdf(20_000)).unwrap();
    assert!(read_pdf_info(file.path()).is_none());
}
