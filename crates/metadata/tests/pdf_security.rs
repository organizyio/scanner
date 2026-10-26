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

#[test]
fn aes_pdf_stream_roundtrip_preserves_padding_boundary_payloads() {
    use lopdf::{
        dictionary,
        encryption::crypt_filters::{Aes128CryptFilter, CryptFilter},
        EncryptionState, EncryptionVersion, Object, Permissions, Stream,
    };
    use std::{collections::BTreeMap, sync::Arc};
    for length in [0, 1, 15, 16, 17, 255] {
        let mut document = lopdf::Document::load_mem(&pdf(1)).unwrap();
        document.trailer.set(
            "ID",
            Object::Array(vec![
                Object::string_literal(b"0123456789abcdef".to_vec());
                2
            ]),
        );
        let payload: Vec<u8> = (0..length).map(|n| (n % 251) as u8).collect();
        let id = document.add_object(Stream::new(dictionary! {}, payload.clone()));
        document
            .get_object_mut((3, 0))
            .unwrap()
            .as_dict_mut()
            .unwrap()
            .set("Contents", id);
        let filter: Arc<dyn CryptFilter> = Arc::new(Aes128CryptFilter);
        let state = EncryptionState::try_from(EncryptionVersion::V4 {
            document: &document,
            encrypt_metadata: true,
            crypt_filters: BTreeMap::from([(b"StdCF".to_vec(), filter)]),
            stream_filter: b"StdCF".to_vec(),
            string_filter: b"StdCF".to_vec(),
            owner_password: "fixture-owner",
            user_password: "fixture-user",
            permissions: Permissions::all(),
        })
        .unwrap();
        document.encrypt(&state).unwrap();
        // Exercise the affected CBC padding path through lopdf's public crypto
        // API. This is not a new encrypted-PDF file support claim.
        document.decrypt("fixture-user").unwrap();
        assert_eq!(
            document
                .get_object(id)
                .unwrap()
                .as_stream()
                .unwrap()
                .content,
            payload,
            "length {length}"
        );
    }
}
